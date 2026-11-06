package routers

import (
	"ehang.io/nps/web/controllers"
	"github.com/astaxie/beego"
)

func registerApiV1(prefix string) {
	type route struct {
		path, mapping string
	}
	routes := []route{
		{"/api/v1/info", "get:Info"},
		{"/api/v1/status", "get:Status"},
		{"/api/v1/clients", "get:Clients"},
		{"/api/v1/client", "get:Client"},
		{"/api/v1/client/create", "post:ClientCreate"},
		{"/api/v1/client/update", "post:ClientUpdate"},
		{"/api/v1/client/status", "post:ClientStatus"},
		{"/api/v1/client/delete", "post:ClientDelete"},
		{"/api/v1/client/remote_update", "post:ClientRemoteUpdate"},
		{"/api/v1/tunnels", "get:Tunnels"},
		{"/api/v1/tunnel", "get:Tunnel"},
		{"/api/v1/tunnel/create", "post:TunnelCreate"},
		{"/api/v1/tunnel/update", "post:TunnelUpdate"},
		{"/api/v1/tunnel/start", "post:TunnelStart"},
		{"/api/v1/tunnel/stop", "post:TunnelStop"},
		{"/api/v1/tunnel/delete", "post:TunnelDelete"},
		{"/api/v1/hosts", "get:Hosts"},
		{"/api/v1/host", "get:Host"},
		{"/api/v1/host/create", "post:HostCreate"},
		{"/api/v1/host/update", "post:HostUpdate"},
		{"/api/v1/host/start", "post:HostStart"},
		{"/api/v1/host/stop", "post:HostStop"},
		{"/api/v1/host/delete", "post:HostDelete"},
		{"/api/v1/users", "get:Users"},
		{"/api/v1/users/create", "post:UserCreate"},
		{"/api/v1/users/update", "post:UserUpdate"},
		{"/api/v1/users/delete", "post:UserDelete"},
		{"/api/v1/users/bind_client", "post:UserBindClient"},
		{"/api/v1/users/api_key", "post:ApiKey"},
		{"/api/v1/audit", "get:Audit"},
	}
	if prefix != "" {
		nsRoutes := make([]beego.LinkNamespace, 0, len(routes)+8)
		nsRoutes = append(nsRoutes,
			beego.NSRouter("/", &controllers.IndexController{}, "*:Index"),
			beego.NSAutoRouter(&controllers.IndexController{}),
			beego.NSAutoRouter(&controllers.LoginController{}),
			beego.NSAutoRouter(&controllers.ClientController{}),
			beego.NSAutoRouter(&controllers.UserController{}),
			beego.NSAutoRouter(&controllers.AuditPanelController{}),
				beego.NSAutoRouter(&controllers.ServerConfigController{}),
				beego.NSAutoRouter(&controllers.WafController{}),
				beego.NSAutoRouter(&controllers.TopologyController{}),
				beego.NSAutoRouter(&controllers.TerminalController{}),
				beego.NSAutoRouter(&controllers.AuthController{}),
				beego.NSRouter("/auth/ipwhiteauth", &controllers.AuthController{}, "post:IpWhiteAuth"),
			)
		for _, r := range routes {
			nsRoutes = append(nsRoutes, beego.NSRouter(r.path, &controllers.ApiV1Controller{}, r.mapping))
		}
		nsRoutes = append(nsRoutes, beego.NSAutoRouter(&controllers.GlobalController{}))
		beego.AddNamespace(beego.NewNamespace(prefix, nsRoutes...))
		return
	}
	beego.Router("/", &controllers.IndexController{}, "*:Index")
	beego.AutoRouter(&controllers.IndexController{})
	beego.AutoRouter(&controllers.LoginController{})
	beego.AutoRouter(&controllers.ClientController{})
	beego.AutoRouter(&controllers.UserController{})
	beego.AutoRouter(&controllers.AuditPanelController{})
	beego.AutoRouter(&controllers.ServerConfigController{})
	beego.AutoRouter(&controllers.WafController{})
	beego.AutoRouter(&controllers.TopologyController{})
	beego.AutoRouter(&controllers.TerminalController{})
	beego.AutoRouter(&controllers.AuthController{})
	beego.Router("/auth/ipwhiteauth", &controllers.AuthController{}, "post:IpWhiteAuth")
	for _, r := range routes {
		beego.Router(r.path, &controllers.ApiV1Controller{}, r.mapping)
	}
	beego.AutoRouter(&controllers.GlobalController{})
}

func Init() {
	// The captcha image path and its filter are built from web_base_url, so it
	// has to be registered here (after the config is loaded) instead of in an
	// init() function.
	controllers.InitCaptcha()
	controllers.RegisterSecureEntryRoutes()
	registerApiV1(beego.AppConfig.String("web_base_url"))
}
