// The site's search, on /search/. The build writes /search.json, with
// each page's title, section, description, and headings; this script
// matches every word of the query against them and lists the pages,
// with links to the headings that matched. Without JavaScript the page
// lists every page instead.
(function () {
  var form = document.querySelector(".search-form");
  var input = document.getElementById("search-q");
  var list = document.getElementById("search-list");
  var status = document.getElementById("search-status");
  var all = document.getElementById("all-pages");
  if (!form || !input || !list || !status) return;

  var index = null;
  var loading = null;
  var timer = 0;

  function load() {
    if (!loading) {
      loading = fetch("/search.json")
        .then(function (r) {
          if (!r.ok) throw new Error(r.status);
          return r.json();
        })
        .then(function (data) {
          index = data.map(function (e) {
            return {
              e: e,
              t: fold(e.t),
              d: fold(e.d),
              s: fold(e.s || ""),
              h: (e.h || []).map(function (h) { return fold(h[1]); })
            };
          });
        });
    }
    return loading;
  }

  // fold lowercases text and straightens curly quotes, so "model's"
  // matches the title's "model’s".
  function fold(s) {
    return s.toLowerCase().replace(/[‘’]/g, "'").replace(/[“”]/g, '"');
  }

  function words(q) {
    return fold(q).split(/[^a-z0-9'_.+-]+/).filter(function (w) { return w.length > 0; });
  }

  // score is 0 unless every word appears somewhere in the page; a word
  // in the title counts most, and one at the start of a word more.
  function score(x, ws) {
    var total = 0;
    for (var i = 0; i < ws.length; i++) {
      var w = ws[i], s = 0, best = 0;
      s += hit(x.t, w) * 10;
      for (var j = 0; j < x.h.length; j++) best = Math.max(best, hit(x.h[j], w));
      s += best * 3;
      s += hit(x.d, w) * 2;
      s += hit(x.s, w);
      if (s === 0) return 0;
      total += s;
    }
    return total;
  }

  function hit(text, w) {
    var at = text.indexOf(w);
    if (at < 0) return 0;
    return at === 0 || /[^a-z0-9]/.test(text.charAt(at - 1)) ? 1.5 : 1;
  }

  function el(tag, cls, text) {
    var n = document.createElement(tag);
    if (cls) n.className = cls;
    if (text) n.textContent = text;
    return n;
  }

  function show(q) {
    var ws = words(q);
    list.textContent = "";
    if (ws.length === 0) {
      list.hidden = true;
      status.textContent = "";
      if (all) all.hidden = false;
      return;
    }
    var found = [];
    for (var i = 0; i < index.length; i++) {
      var sc = score(index[i], ws);
      if (sc > 0) found.push({ x: index[i], sc: sc });
    }
    found.sort(function (a, b) { return b.sc - a.sc; });
    found.forEach(function (f) {
      var e = f.x.e;
      var li = el("li");
      var a = el("a", "result-title", e.t);
      a.href = e.u;
      li.appendChild(a);
      if (e.s) li.appendChild(el("span", "result-section", e.s));
      li.appendChild(el("p", "result-desc", e.d));
      var hs = [];
      for (var j = 0; j < f.x.h.length && hs.length < 3; j++) {
        var h = f.x.h[j];
        if (ws.some(function (w) { return h.indexOf(w) >= 0; })) hs.push(e.h[j]);
      }
      if (hs.length > 0) {
        var ul = el("ul", "result-headings");
        hs.forEach(function (h) {
          var hl = el("a", "", h[1]);
          hl.href = e.u + "#" + h[0];
          var hi = el("li");
          hi.appendChild(hl);
          ul.appendChild(hi);
        });
        li.appendChild(ul);
      }
      list.appendChild(li);
    });
    list.hidden = found.length === 0;
    if (all) all.hidden = found.length > 0;
    status.textContent = found.length === 0
      ? "No page matches “" + q + "”. Try fewer or shorter words, or browse every page below."
      : found.length + (found.length === 1 ? " page matches" : " pages match") + " “" + q + "”.";
  }

  function run() {
    var q = input.value.trim();
    var url = q ? "?q=" + encodeURIComponent(q) : location.pathname;
    try { history.replaceState(null, "", url); } catch (e) {}
    if (!q) { show(""); return; }
    load().then(function () { show(q); }, function () {
      status.textContent = "The search index didn’t load. Every page is listed below.";
    });
  }

  form.addEventListener("submit", function (e) {
    e.preventDefault();
    run();
  });
  input.addEventListener("input", function () {
    clearTimeout(timer);
    timer = setTimeout(run, 120);
  });

  // The arrow keys move between the box and the results' titles.
  function titles() { return list.querySelectorAll(".result-title"); }
  input.addEventListener("keydown", function (e) {
    if (e.key === "ArrowDown" && !list.hidden) {
      var t = titles();
      if (t.length) { e.preventDefault(); t[0].focus(); }
    } else if (e.key === "Escape" && input.value) {
      input.value = "";
      run();
    }
  });
  list.addEventListener("keydown", function (e) {
    if (e.key !== "ArrowDown" && e.key !== "ArrowUp") return;
    var t = Array.prototype.slice.call(titles());
    var i = t.indexOf(document.activeElement);
    if (i < 0) return;
    e.preventDefault();
    if (e.key === "ArrowDown" && i + 1 < t.length) t[i + 1].focus();
    else if (e.key === "ArrowUp") (i > 0 ? t[i - 1] : input).focus();
  });

  var q = new URLSearchParams(location.search).get("q");
  if (q) {
    input.value = q;
    run();
  }
  input.focus();
})();
