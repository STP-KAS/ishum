(function () {
  var amount = 0;
  var raw = "";
  var tipPct = 0;
  var lines = [];
  var mode = "keypad";
  var ccy = (document.getElementById("charge-btn") || {}).textContent || "EUR";
  ccy = (ccy.match(/[A-Z]{3}/) || ["EUR"])[0];

  function money(n) {
    return n.toFixed(2);
  }
  function total() {
    var base = mode === "cart"
      ? lines.reduce(function (s, l) { return s + l.price * l.n; }, 0)
      : amount;
    return { base: base, tip: base * tipPct / 100, all: base * (1 + tipPct / 100) };
  }
  function paint() {
    var t = total();
    var d = document.getElementById("display");
    if (d) d.innerHTML = money(t.all) + " <small>" + ccy + "</small>";
    document.getElementById("amount").value = money(t.base);
    document.getElementById("tip").value = money(t.tip);
    var names = mode === "cart"
      ? lines.map(function (l) { return l.n + "× " + l.title; }).join(", ")
      : "sale";
    document.getElementById("item").value = names || "sale";
    var btn = document.getElementById("charge-btn");
    btn.textContent = "Charge " + money(t.all) + " " + ccy;
    btn.disabled = t.base <= 0;
    var ul = document.getElementById("lines");
    if (ul) {
      ul.innerHTML = lines.map(function (l) {
        return "<li><span>" + l.n + " × " + l.title + "</span><strong>" + money(l.price * l.n) + "</strong></li>";
      }).join("");
    }
  }
  function setMode(m) {
    mode = m;
    document.getElementById("keypad").classList.toggle("hide", m !== "keypad");
    document.getElementById("cart").classList.toggle("hide", m !== "cart");
    document.getElementById("mode-keypad").classList.toggle("on", m === "keypad");
    document.getElementById("mode-cart").classList.toggle("on", m === "cart");
    paint();
  }
  document.getElementById("mode-keypad").onclick = function () { setMode("keypad"); };
  document.getElementById("mode-cart").onclick = function () { setMode("cart"); };

  document.getElementById("keypad").addEventListener("click", function (e) {
    var k = e.target.getAttribute("data-k");
    if (!k) return;
    if (k === "c") { raw = ""; amount = 0; }
    else if (k === "." && raw.indexOf(".") >= 0) return;
    else {
      if (raw.indexOf(".") >= 0 && raw.split(".")[1].length >= 2) return;
      raw += k;
      amount = parseFloat(raw) || 0;
    }
    paint();
  });

  document.getElementById("cart").addEventListener("click", function (e) {
    var b = e.target.closest(".item");
    if (!b) return;
    var title = b.getAttribute("data-title");
    var price = parseFloat(b.getAttribute("data-price"));
    var found = lines.filter(function (l) { return l.title === title; })[0];
    if (found) found.n += 1;
    else lines.push({ title: title, price: price, n: 1 });
    paint();
  });

  document.querySelector(".tips").addEventListener("click", function (e) {
    var t = e.target.getAttribute("data-tip");
    if (t == null) return;
    tipPct = parseInt(t, 10);
    [].forEach.call(document.querySelectorAll(".tips button"), function (b) {
      b.classList.toggle("on", b.getAttribute("data-tip") === t);
    });
    paint();
  });

  document.getElementById("charge").addEventListener("submit", function (e) {
    if (total().base <= 0) e.preventDefault();
  });
  paint();
})();
