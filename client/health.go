package client

import (
	"container/heap"
	"io"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"ehang.io/nps/lib/conn"
	"ehang.io/nps/lib/file"
	"ehang.io/nps/lib/sheap"
	"github.com/astaxie/beego/logs"
	"github.com/pkg/errors"
)

var (
	isStart bool
	// serverConn 会被 heathCheck（重连时）写、check（检查 goroutine）读。
	serverConnMu sync.RWMutex
	serverConn   *conn.Conn
	// signalWriteMu 保护 WORK_MAIN 上健康上报与远程升级回传的并发写。
	signalWriteMu sync.Mutex
)

func setServerConn(c *conn.Conn) {
	serverConnMu.Lock()
	serverConn = c
	serverConnMu.Unlock()
}

func getServerConn() *conn.Conn {
	serverConnMu.RLock()
	defer serverConnMu.RUnlock()
	return serverConn
}

func heathCheck(healths []*file.Health, c *conn.Conn) bool {
	setServerConn(c)
	if isStart {
		for _, v := range healths {
			v.Lock()
			v.HealthMap = make(map[string]int)
			v.Unlock()
		}
		return true
	}
	isStart = true
	h := &sheap.IntHeap{}
	for _, v := range healths {
		if v.HealthMaxFail > 0 && v.HealthCheckTimeout > 0 && v.HealthCheckInterval > 0 {
			v.HealthNextTime = time.Now().Add(time.Duration(v.HealthCheckInterval) * time.Second)
			heap.Push(h, v.HealthNextTime.Unix())
			v.Lock()
			v.HealthMap = make(map[string]int)
			v.Unlock()
		}
	}
	go session(healths, h)
	return true
}

func session(healths []*file.Health, h *sheap.IntHeap) {
	for {
		if h.Len() == 0 {
			logs.Error("health check error")
			break
		}
		rs := heap.Pop(h).(int64) - time.Now().Unix()
		if rs <= 0 {
			// 已到期：尽快跑，并照常把下一次排进堆（勿 continue 丢任务）。
			rs = 1
		}
		timer := time.NewTimer(time.Duration(rs) * time.Second)
		<-timer.C
		timer.Stop()
		for _, v := range healths {
			if v.HealthNextTime.Before(time.Now()) {
				v.HealthNextTime = time.Now().Add(time.Duration(v.HealthCheckInterval) * time.Second)
				go check(v)
				heap.Push(h, v.HealthNextTime.Unix())
			}
		}
	}
}

// work when just one port and many target
func check(t *file.Health) {
	arr := strings.Split(t.HealthCheckTarget, ",")
	var err error
	var rs *http.Response
	for _, v := range arr {
		v = strings.TrimSpace(v)
		if v == "" {
			continue
		}
		if t.HealthCheckType == "tcp" {
			var c net.Conn
			c, err = net.DialTimeout("tcp", v, time.Duration(t.HealthCheckTimeout)*time.Second)
			if err == nil {
				_ = c.Close()
			}
		} else {
			scheme := "http"
			if strings.EqualFold(t.HealthCheckType, "https") {
				scheme = "https"
			}
			client := &http.Client{Timeout: time.Duration(t.HealthCheckTimeout) * time.Second}
			rs, err = client.Get(scheme + "://" + v + t.HttpHealthUrl)
			if rs != nil {
				_, _ = io.Copy(io.Discard, rs.Body)
				_ = rs.Body.Close()
			}
			if err == nil && rs != nil && rs.StatusCode != 200 {
				err = errors.New("status code is not match")
			}
		}
		t.Lock()
		if err != nil {
			t.HealthMap[v] += 1
		} else {
			// 成功一次清零；若此前已判下线则上报恢复。
				if t.HealthMap[v] >= t.HealthMaxFail {
					if sc := getServerConn(); sc != nil {
						signalWriteMu.Lock()
						_, _ = sc.SendHealthInfo(v, "1")
						signalWriteMu.Unlock()
					}
				}
				t.HealthMap[v] = 0
			}

			if t.HealthMap[v] > 0 && t.HealthMap[v]%t.HealthMaxFail == 0 {
				if sc := getServerConn(); sc != nil {
					signalWriteMu.Lock()
					_, _ = sc.SendHealthInfo(v, "0")
					signalWriteMu.Unlock()
				}
			}
		t.Unlock()
	}
}
