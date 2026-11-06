(function ($) {

	function xml2json(Xml) {
		var tempvalue, tempJson = {};
		$(Xml).each(function() {
			var tagName = ($(this).attr('id') || this.tagName);
			tempvalue = (this.childElementCount == 0) ? this.textContent : xml2json($(this).children());
			switch ($.type(tempJson[tagName])) {
				case 'undefined':
					tempJson[tagName] = tempvalue;
					break;
				case 'object':
					tempJson[tagName] = Array(tempJson[tagName]);
				case 'array':
					tempJson[tagName].push(tempvalue);
			}
		});
		return tempJson;
	}

	function setCookie (c_name, value, expiredays) {
		var exdate = new Date();
		exdate.setDate(exdate.getDate() + expiredays);
		document.cookie = c_name + '=' + escape(value) + ((expiredays == null) ? '' : ';expires=' + exdate.toGMTString())+ '; path='+window.nps.web_base_url+'/;';
	}

	function getCookie (c_name) {
		if (document.cookie.length > 0) {
			c_start = document.cookie.indexOf(c_name + '=');
			if (c_start != -1) {
				c_start = c_start + c_name.length + 1;
				c_end = document.cookie.indexOf(';', c_start);
				if (c_end == -1) c_end = document.cookie.length;
				return unescape(document.cookie.substring(c_start, c_end));
			}
		}
		return null;
	}

	function setchartlang (langobj,chartobj) {
		if ( $.type (langobj) == 'string' ) return langobj;
		if ( $.type (langobj) == 'chartobj' ) return false;
		var flag = true;
		for (key in langobj) {
			var item = key;
			children = (chartobj.hasOwnProperty(item)) ? setchartlang (langobj[item],chartobj[item]) : setchartlang (langobj[item],undefined);
			switch ($.type(children)) {
				case 'string':
					if ($.type(chartobj[item]) != 'string' ) continue;
				case 'object':
					chartobj[item] = (children['value'] || children);
				default:
					flag = false;
			}
		}
		if (flag) { return {'value':(langobj[languages['current']] || langobj[languages['default']] || 'N/A')}}
	}

	// Match a browser language (e.g. "zh", "zh-TW", "en-GB") to an available
	// menu language key (e.g. "zh-CN", "en-US"). Exact match wins, then the
	// primary subtag (before the first "-"). Returns null when nothing matches
	// so the caller can fall back to the default language.
	function normalizeLang (lang) {
		if (!lang) return null;
		var lower = ('' + lang).toLowerCase();
		var primary = lower.split('-')[0];
		var fallback = null;
		for (var key in languages['menu']) {
			var k = key.toLowerCase();
			if (k === lower) return key;
			if (fallback == null && k.split('-')[0] === primary) fallback = key;
		}
		return fallback;
	}

	$.fn.cloudLang = function () {
		$.ajax({
			type: 'GET',
			url: window.nps.web_base_url + '/static/page/languages.xml?v=' + window.nps.version,
			dataType: 'xml',
			success: function (xml) {
				languages['content'] = xml2json($(xml).children())['content'];
				languages['menu'] = languages['content']['languages'];
				languages['default'] = languages['content']['default'];
				languages['navigator'] = (getCookie ('lang') || normalizeLang(navigator.language || navigator.browserLanguage) || languages['default']);
				// 清空模板里的无图标占位项，只保留带国旗图标的语言列表
				$('#languagemenu').next().empty();
				for(var key in languages['menu']){
					$('#languagemenu').next().append('<li lang="' + key + '"><a class="dropdown-item" href="#"><img src="' + window.nps.web_base_url + '/static/img/flag/' + key + '.png" alt=""> ' + languages['menu'][key] +'</a></li>');
					if ( key == languages['navigator'] ) languages['current'] = key;
				}
				$('#languagemenu').attr('lang',(languages['current'] || languages['default']));
				$('body').setLang ('');
			}
		});
	};

	$.fn.setLang = function (dom, persistLang) {
		languages['current'] = $('#languagemenu').attr('lang');
		var scope = (dom == null) ? '' : String(dom);
		if (scope === '') {
			$('#languagemenu span').text(' ' + languages['menu'][languages['current']]);
			if (persistLang && languages['current'] != getCookie('lang')) setCookie('lang', languages['current'], 36500);
			if($("#table").length>0) {
				var $tbl = $('#table');
				if ($tbl.data('nps.table') || $tbl.data('npsTable')) {
					$tbl.npsTable('refresh');
				}
			}
		}
		var langRoot = scope ? (scope + ' ') : '';
		var appName = (window.nps && window.nps.app_name) ? String(window.nps.app_name).trim() : '';
		$.each($(langRoot + '[langtag]'), function (i, item) {
			var index = $(item).attr('langtag');
			var key = String(index).toLowerCase();
			// 后台可改产品名：覆盖语言包 application；标题里的 NPS 一并替换
			if (key === 'application' && appName) {
				string = appName;
			} else {
			string = languages['content'][key];
			switch ($.type(string)) {
				case 'string':
					break;
				case 'array':
					string = string[Math.floor((Math.random()*string.length))];
				case 'object':
					string = (string[languages['current']] || string[languages['default']] || null);
					break;
				default:
					string = 'Missing language string "' + index + '"';
					$(item).css('background-color','#ffeeba');
			}
			if (appName && typeof string === 'string' && key.indexOf('title-') === 0) {
				string = string.replace(/\bNPS\b/g, appName);
			}
			}
			if($.type($(item).attr('placeholder')) == 'undefined') {
				$(item).text(string);
			} else {
				$(item).attr('placeholder', string);
			}
			// <title langtag> 同步浏览器标题栏
			if (item.tagName && String(item.tagName).toLowerCase() === 'title' && typeof string === 'string') {
				document.title = string;
			}
		});

		var locale = languages['current'] === 'en-US' ? 'en' : 'zh';
		$.each($(langRoot + '[data-i18n-zh]'), function (i, item) {
			var text = $(item).attr('data-i18n-' + locale);
			if ($.type(text) === 'string' && $.trim(text) !== '') $(item).text(text);
		});
		$.each($(langRoot + '[data-placeholder-zh]'), function (i, item) {
			var text = $(item).attr('data-placeholder-' + locale);
			if ($.type(text) === 'string' && $.trim(text) !== '') $(item).attr('placeholder', text);
		});
		$.each($(langRoot + '[data-aria-label-zh]'), function (i, item) {
			var text = $(item).attr('data-aria-label-' + locale);
			if ($.type(text) === 'string' && $.trim(text) !== '') $(item).attr('aria-label', text);
		});
		$.each($(langRoot + '[data-title-zh]'), function (i, item) {
			var text = $(item).attr('data-title-' + locale);
			if ($.type(text) === 'string' && $.trim(text) !== '') $(item).attr('title', text);
		});

		if ( !$.isEmptyObject(chartdatas) ) {
			setchartlang(languages['content']['charts'],chartdatas);
			for(var key in chartdatas){
				if ($('#'+key).length == 0) continue;
				if($.type(chartdatas[key]) == 'object')
				charts[key] = echarts.init(document.getElementById(key));
				charts[key].setOption(chartdatas[key], true);
			}
		}
	}

	// 表格异步渲染后局部套用文案（勿传空串，空串会触发整表 refresh）
	window.npsApplyLang = function (scope) {
		if (!languages || !languages['content']) return;
		$('body').setLang(scope || 'body', false);
	};

})(jQuery);

$(document).ready(function () {
	$('body').cloudLang();
	$('body').on('click','li[lang]',function(){
		$('#languagemenu').attr('lang',$(this).attr('lang'));
		$('body').setLang ('', true);
	});
	$('body').on('input change', '[data-required="true"]', function () {
		$(this).removeClass('is-invalid');
	});
});

var languages = {};
var charts = {};
var chartdatas = {};
var postsubmit;

function langreply(langstr) {
    if (!languages || !languages['content'] || !languages['content']['reply']) return langstr;
    var langobj = languages['content']['reply'][langstr.replace(/[\s,\.\?]*/g,"").toLowerCase()];
    if ($.type(langobj) == 'undefined') return langstr
    langobj = (langobj[languages['current']] || langobj[languages['default']] || langstr);
    return langobj
}

function validateRequiredFields(formSelector) {
    var $form = $(formSelector || 'form').first();
    if ($form.length === 0) return true;

    var $fields = $form.find('[data-required="true"]').filter(':visible');
    $fields.removeClass('is-invalid');
    $form.find('.nps-form-error').remove();

    var $invalid = $fields.filter(function () {
        var $el = $(this);
        var val = $.trim($el.val());
        if (val === '') return true;
        var minLen = parseInt($el.attr('data-minlength') || '0', 10);
        if (minLen > 0 && val.length < minLen) return true;
        var pattern = $el.attr('data-pattern');
        if (pattern) {
            try {
                if (!(new RegExp(pattern)).test(val)) return true;
            } catch (e) {}
        }
        return false;
    });
    if ($invalid.length === 0) return true;

    $invalid.addClass('is-invalid');
    var msg = 'Please fill in the required fields';
    if (languages && languages['content'] && languages['content']['info-requiredfields']) {
        var requiredMsg = languages['content']['info-requiredfields'];
        msg = requiredMsg[languages['current']] || requiredMsg[languages['default']] || msg;
    }
    if (typeof window.npsShowFormError === 'function') {
        window.npsShowFormError($form, msg);
    } else {
        alert(msg);
    }
    var $first = $invalid.first();
    if ($first.length && $first[0].scrollIntoView) {
        $first[0].scrollIntoView({ behavior: 'smooth', block: 'center' });
    }
    $first.focus();
    return false;
}

var nps_submitting = false;
var nps_batch_submitting = false;
function submitform(action, url, postdata) {
    if (nps_submitting) return;
    if ((action === 'add' || action === 'edit') && !validateRequiredFields('form')) return;
    postsubmit = false;
    switch (action) {
        case 'start':
        case 'stop':
        case 'delete':
		case 'copy':
		case 'upgrade':
            var confirmObj = (languages && languages['content'] && languages['content']['confirm']) ? languages['content']['confirm'][action] : null;
            var confirmMsg = (confirmObj && (confirmObj[languages['current']] || confirmObj[languages['default']])) || ('Are you sure you want to ' + action + ' it?');
            if (! confirm(confirmMsg)) return;
            postsubmit = true;
        case 'add':
        case 'edit':
            nps_submitting = true;
            $.ajax({
                type: "POST",
                url: url,
                data: postdata,
                // 远程升级会等待客户端下载回传，最长约 120s
                timeout: (action === 'upgrade') ? 130000 : 0,
                success: function (res) {
                    alert(langreply(res.msg));
                    if (res.status) {
                        if (postsubmit) {
							document.location.reload();
						}else{
							window.location.href= document.referrer
						}
                    }
                },
                error: function (xhr, textStatus) {
                    if (action === 'upgrade' && textStatus === 'timeout') {
                        alert('等待客户端升级回传超时；指令可能已下发，请查看客户端日志或刷新列表');
                    }
                },
                complete: function () {
                    nps_submitting = false;
                }
            });
			return;
		case 'global':
			nps_submitting = true;
			$.ajax({
				type: "POST",
				url: url,
				data: postdata,
				success: function (res) {
					alert(langreply(res.msg));
					if (res.status) {
						document.location.reload();
					}
				},
				complete: function () {
					nps_submitting = false;
				}
			});
    }
}

function changeunit(limit) {
    var size = "";
    if (limit < 0.1 * 1024) {
        size = limit.toFixed(2) + "B";
    } else if (limit < 0.1 * 1024 * 1024) {
        size = (limit / 1024).toFixed(2) + "KB";
    } else if (limit < 0.1 * 1024 * 1024 * 1024) {
        size = (limit / (1024 * 1024)).toFixed(2) + "MB";
    } else {
        size = (limit / (1024 * 1024 * 1024)).toFixed(2) + "GB";
    }

    var sizeStr = size + "";
    var index = sizeStr.indexOf(".");
    var dou = sizeStr.substr(index + 1, 2);
    if (dou == "00") {
        return sizeStr.substring(0, index) + sizeStr.substr(index + 3, 2);
    }
    return size;
}

	function npsTableGetSelections() {
	    var $tbl = $('#table');
	    if ($tbl.data('nps.table') || $tbl.data('npsTable')) {
	        return $tbl.npsTable('getSelections') || [];
	    }
	    return [];
	}

function batchDelete(url) {
    var rows = npsTableGetSelections();
    if (rows.length === 0) {
        alert(languages && languages['content'] && languages['content']['confirm'] && languages['content']['confirm']['noselected']
            ? (languages['content']['confirm']['noselected'][languages['current']] || languages['content']['confirm']['noselected'][languages['default']] || 'Please select items to delete.')
            : 'Please select items to delete.');
        return;
    }
    var confirmObj = (languages && languages['content'] && languages['content']['confirm']) ? languages['content']['confirm']['delete'] : null;
    var confirmMsg = (confirmObj && (confirmObj[languages['current']] || confirmObj[languages['default']])) || ('Are you sure you want to delete ' + rows.length + ' items?');
    if (!confirm(confirmMsg + ' (' + rows.length + ' ' + (rows.length > 1 ? 'items' : 'item') + ')')) return;
    if (nps_batch_submitting) return;
    nps_batch_submitting = true;
    var ids = [];
    for (var i = 0; i < rows.length; i++) {
        ids.push(rows[i].Id);
    }
    var idx = 0;
    function next() {
        if (idx >= ids.length) {
            nps_batch_submitting = false;
            document.location.reload();
            return;
        }
        $.ajax({
            type: "POST",
            url: url,
            data: { id: ids[idx] },
            success: function (res) {
                idx++;
                next();
            },
            error: function () {
                idx++;
                next();
            }
        });
    }
    next();
}