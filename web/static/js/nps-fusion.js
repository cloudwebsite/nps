/* 面板渐进辅助（徽章、详情 chips、工具栏、密度、showMsg） */
(function (window, $) {
  "use strict";
  if (!$ || !window) return;

  var DENSITY_KEY = "nps_ui_density";
  var THEME_KEY = "nps_ui_theme";

  // WAF / 证书目录 / 全局参数等页共用；基于已加载的 toastr
  window.showMsg = function (text, type, dur, cb) {
    type = type || "success";
    dur = dur == null ? 1500 : dur;
    var msg = text == null ? "" : String(text);
    function done() {
      if (typeof cb === "function") {
        try { cb(); } catch (e) {}
      }
    }
    if (window.toastr) {
      var opts = { timeOut: dur, extendedTimeOut: Math.max(dur, 1000), closeButton: true };
      if (type === "error") {
        toastr.error(msg, "", opts);
      } else if (type === "warning") {
        toastr.warning(msg, "", opts);
      } else if (type === "loading" || type === "info") {
        toastr.info(msg, "", opts);
      } else {
        toastr.success(msg, "", opts);
      }
      if (typeof cb === "function") {
        setTimeout(done, dur);
      }
      return;
    }
    try { window.alert(msg); } catch (e) {}
    done();
  };

  function escapeHtml(text) {
    return String(text == null ? "" : text)
      .replace(/&/g, "&amp;")
      .replace(/</g, "&lt;")
      .replace(/>/g, "&gt;")
      .replace(/"/g, "&quot;");
  }

  window.npsBadge = function (text, tone) {
    var cls = "nps-badge nps-badge-muted";
    if (tone === "success") cls = "nps-badge nps-badge-success";
    if (tone === "danger") cls = "nps-badge nps-badge-danger";
    if (tone === "info") cls = "nps-badge nps-badge-info";
    if (tone === "warning") cls = "nps-badge nps-badge-warning";
    return '<span class="' + cls + '">' + escapeHtml(text) + "</span>";
  };

  window.npsResultBadge = function (result) {
    var r = String(result || "").toLowerCase();
    if (r === "success") return window.npsBadge("success", "success");
    if (r === "failure") return window.npsBadge("failure", "danger");
    return window.npsBadge(result || "-", "muted");
  };

  window.npsOnlineBadge = function (online, onlineText, offlineText) {
    if (online) return window.npsBadge(onlineText || "online", "success");
    return window.npsBadge(offlineText || "offline", "muted");
  };

  window.npsOpenBadge = function (open, openText, closeText) {
    if (open) return window.npsBadge(openText || "open", "success");
    return window.npsBadge(closeText || "close", "muted");
  };

  window.npsDetailChips = function (details) {
    if (!details || typeof details !== "object") return "";
    var preferred = ["addr", "transport", "version", "local_addr", "client_id"];
    var parts = [];
    preferred.forEach(function (key) {
      if (details[key]) {
        parts.push('<span class="nps-detail-chip">' + escapeHtml(key) + "=" + escapeHtml(details[key]) + "</span>");
      }
    });
    Object.keys(details).forEach(function (key) {
      if (preferred.indexOf(key) >= 0) return;
      parts.push('<span class="nps-detail-chip">' + escapeHtml(key) + "=" + escapeHtml(details[key]) + "</span>");
    });
    return parts.join("");
  };

  window.npsBindToolbarRefresh = function (tableSelector, selectorList) {
    $(selectorList.join(",")).on("change keyup", function (e) {
      if (e.type === "keyup" && e.keyCode !== 13) return;
      var $t = $(tableSelector);
      if ($t.data("nps.table") || $t.data("npsTable")) {
        $t.npsTable("refresh");
      }
    });
  };

  function densityLabel(density) {
    if (density === "compact") {
      return (window.langreply && langreply("comfortable")) || "舒适 / Comfortable";
    }
    return (window.langreply && langreply("compact")) || "紧凑 / Compact";
  }

  window.npsApplyDensity = function (mode) {
    var density = mode === "compact" ? "compact" : "comfortable";
    try {
      window.localStorage.setItem(DENSITY_KEY, density);
    } catch (e) {}
    $("body").toggleClass("nps-density-compact", density === "compact");
    $(".nps-density-toggle").each(function () {
      $(this).text(density === "compact" ? "舒适" : "紧凑");
      $(this).attr("data-density", density);
      $(this).attr("title", densityLabel(density));
    });
  };

  window.npsApplyTheme = function (mode) {
    var theme = mode === "dark" ? "dark" : "light";
    try {
      window.localStorage.setItem(THEME_KEY, theme);
      window.localStorage.setItem("nps-theme", theme);
    } catch (e) {}
    document.documentElement.setAttribute("data-theme", theme);
    document.documentElement.classList.toggle("nps-theme-dark", theme === "dark");
    $("body").toggleClass("nps-theme-dark", theme === "dark");
    $(".nps-theme-toggle").each(function () {
      var $icon = $(this).find("i");
      if ($icon.length) {
        $icon.attr("class", theme === "dark" ? "fa fa-sun" : "fa fa-moon");
      }
      $(this).attr("data-theme", theme);
      $(this).attr("title", theme === "dark" ? "Light" : "Dark");
      $(this).attr("aria-pressed", theme === "dark" ? "true" : "false");
    });
  };

  function clientLabel(row) {
    var id = row && (row.Id != null ? row.Id : row.id);
    var remark = (row && (row.Remark != null ? row.Remark : row.remark)) || "";
    var online = !!(row && (row.IsConnect != null ? row.IsConnect : row.is_connect));
    var name = remark || ("client-" + id);
    return "#" + id + " · " + name + (online ? " ●" : " ○");
  }

  function sortClientsOnlineFirst(rows) {
    return (rows || []).slice().sort(function (a, b) {
      var ao = a && (a.IsConnect != null ? a.IsConnect : a.is_connect) ? 1 : 0;
      var bo = b && (b.IsConnect != null ? b.IsConnect : b.is_connect) ? 1 : 0;
      if (ao !== bo) return bo - ao;
      var ai = a && (a.Id != null ? a.Id : a.id);
      var bi = b && (b.Id != null ? b.Id : b.id);
      return (Number(ai) || 0) - (Number(bi) || 0);
    });
  }

  // 表单客户端下拉：搜索筛选 + 多条目时改为列表高度，在线优先
  window.npsEnhanceClientSelect = function (selector) {
    var $sel = $(selector);
    if (!$sel.length || $sel.data("npsClientSelectReady")) return $sel;
    $sel.data("npsClientSelectReady", 1);
    var $wrap = $('<div class="nps-client-select"></div>');
    $sel.before($wrap);
    $wrap.append($sel);
    var $search = $(
      '<input type="search" class="form-control nps-client-select__search" autocomplete="off" spellcheck="false" placeholder="筛选 ID / 备注…">'
    );
    $sel.before($search);
    var cache = [];
    function snapshot() {
      cache = $sel.find("option").map(function () {
        var $o = $(this);
        return {
          value: String($o.val()),
          text: $o.text(),
          online: $o.attr("data-online") === "1",
          remark: $o.attr("data-remark") || ""
        };
      }).get();
      $wrap.toggleClass("nps-client-select--many", cache.length > 10);
      if (cache.length > 10) {
        $sel.attr("size", Math.min(12, cache.length));
      } else {
        $sel.removeAttr("size");
      }
      $search.toggle(cache.length > 6);
    }
    function refill(q) {
      q = String(q || "").trim().toLowerCase();
      var cur = String($sel.val() || "");
      $sel.empty();
      cache.forEach(function (item) {
        var hay = (item.value + " " + item.text + " " + item.remark).toLowerCase();
        if (q && hay.indexOf(q) < 0) return;
        $("<option></option>")
          .val(item.value)
          .text(item.text)
          .attr("data-online", item.online ? "1" : "0")
          .attr("data-remark", item.remark)
          .appendTo($sel);
      });
      var hasCur = $sel.find("option").filter(function () {
        return String(this.value) === cur;
      }).length > 0;
      if (hasCur) {
        $sel.val(cur);
      } else if ($sel.find("option").length) {
        $sel.prop("selectedIndex", 0);
      }
    }
    snapshot();
    $sel.data("npsClientSelectSnapshot", snapshot);
    $search.on("input", function () {
      refill(this.value);
    });
    return $sel;
  };

  window.npsLoadClientSelect = function (selector, selectedId) {
    var $sel = $(selector);
    if (!$sel.length) return $.Deferred().resolve().promise();
    var base = (window.nps && window.nps.web_base_url) || "";
    return $.ajax({
      type: "POST",
      url: base + "/client/list",
      data: { order: "asc", offset: 0, limit: 2000 },
      dataType: "json"
    }).done(function (data) {
      var rows = sortClientsOnlineFirst((data && data.rows) || []);
      $sel.empty();
      rows.forEach(function (row) {
        var id = row.Id;
        var online = !!row.IsConnect;
        $("<option></option>")
          .val(id)
          .text(clientLabel(row))
          .attr("data-online", online ? "1" : "0")
          .attr("data-remark", row.Remark || "")
          .appendTo($sel);
      });
      if (selectedId != null && selectedId !== "" && Number(selectedId) > 0) {
        $sel.val(String(selectedId));
      }
      if ($sel.data("npsClientSelectReady")) {
        var snap = $sel.data("npsClientSelectSnapshot");
        if (typeof snap === "function") snap();
        $sel.siblings(".nps-client-select__search").val("");
      } else {
        window.npsEnhanceClientSelect($sel);
      }
    });
  };

  window.npsShowFormError = function ($form, msg) {
    if (!$form || !$form.length) return;
    $form.find('.nps-form-error').remove();
    var $banner = $('<div class="alert alert-danger nps-form-error" role="alert"></div>').text(msg || "");
    $form.prepend($banner);
    setTimeout(function () {
      $banner.fadeOut(200, function () { $(this).remove(); });
    }, 5000);
  };

  window.npsInitListToolbar = function (opts) {
    opts = opts || {};
    var $toolbar = $(opts.toolbar || "#toolbar");
    if (!$toolbar.length) return;
    $toolbar.addClass("nps-toolbar-row");
    if (!$toolbar.find(".nps-density-toggle").length) {
      var $btn = $('<button type="button" class="btn btn-default btn-sm nps-density-toggle">紧凑</button>');
      $btn.on("click", function () {
        var cur = $(this).attr("data-density") || "comfortable";
        window.npsApplyDensity(cur === "compact" ? "comfortable" : "compact");
      });
      $toolbar.append($btn);
    }
    var saved = "comfortable";
    try {
      saved = window.localStorage.getItem(DENSITY_KEY) || "comfortable";
    } catch (e) {}
    window.npsApplyDensity(saved);
  };

  // 新增/保存等提交区不得被收进折叠组
  function isFormActionBoundary(el) {
    if (!el) return true;
    if (el.classList && el.classList.contains("form-actions")) return true;
    if (el.classList && el.classList.contains("hr-line-dashed")) {
      var after = el.nextElementSibling;
      if (after && after.classList && after.classList.contains("form-actions")) return true;
    }
    if (el.querySelector) {
      if (el.querySelector(".form-actions")) return true;
      if (el.querySelector('button[type="submit"]')) return true;
      if (el.querySelector('[langtag="word-add"], [langtag="word-save"]')) return true;
    }
    return false;
  }

  function initCollapsibleFormSections(root) {
    if (!root || !root.querySelectorAll) return;
    var forms = Array.prototype.slice.call(root.querySelectorAll("form"));
    forms.forEach(function (form) {
      if (form.getAttribute("data-nps-collapse-ready") === "1") return;
      // 融合仓表单用 .form-section 作分组标题；从第 3 组起默认折叠后续字段
      var headings = Array.prototype.filter.call(form.children, function (child) {
        return child.classList && (child.classList.contains("form-section-title") || child.classList.contains("form-section"));
      });
      if (headings.length < 3) return;
      form.setAttribute("data-nps-collapse-ready", "1");
      headings.forEach(function (heading, index) {
        if (index < 2) return;
        var wrap = document.createElement("div");
        wrap.className = "nps-form-collapse is-collapsed";
        var next = heading.nextElementSibling;
        heading.parentNode.insertBefore(wrap, heading);
        wrap.appendChild(heading);
        // 展开按钮单独成行（整组独占），不要塞进标题里贴到上一组右侧
        var btn = document.createElement("button");
        btn.type = "button";
        btn.className = "btn btn-outline-secondary btn-sm form-section-toggle";
        btn.textContent = "展开本组";
        wrap.appendChild(btn);
        while (
          next &&
          !(next.classList && (next.classList.contains("form-section-title") || next.classList.contains("form-section"))) &&
          !isFormActionBoundary(next)
        ) {
          var cur = next;
          next = next.nextElementSibling;
          wrap.appendChild(cur);
        }
        btn.addEventListener("click", function () {
          var open = !wrap.classList.toggle("is-collapsed");
          btn.textContent = open ? "收起本组" : "展开本组";
        });
      });
    });
  }

  $(function () {
    if ($("#toolbar").length) {
      window.npsInitListToolbar({ toolbar: "#toolbar" });
    }
    var savedTheme = "light";
    try {
      savedTheme = window.localStorage.getItem(THEME_KEY) || localStorage.getItem("nps-theme") || "light";
    } catch (e) {}
    window.npsApplyTheme(savedTheme);
    $(document).on("click", ".nps-theme-toggle", function () {
      var cur = $(this).attr("data-theme") || document.documentElement.getAttribute("data-theme") || "light";
      window.npsApplyTheme(cur === "dark" ? "light" : "dark");
    });
    initCollapsibleFormSections(document);
  });
})(window, window.jQuery);
