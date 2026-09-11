(function () {
  var cfg = window.ISHUM || {};
  function wallet() {
    if (window.kasware && typeof window.kasware.sendKaspa === "function") return window.kasware;
    if (window.kastle && typeof window.kastle.sendKaspa === "function") return window.kastle;
    return null;
  }
  var btn = document.getElementById("kasware");
  if (btn) {
    btn.onclick = function () {
      var w = wallet();
      if (!w) {
        if (cfg.uri) location.href = cfg.uri;
        else alert("No in-page wallet. Open the kaspa: link or scan the QR.");
        return;
      }
      w.requestAccounts().then(function () {
        return w.sendKaspa(cfg.uri ? cfg.uri.split("?")[0] : "", String(cfg.sompi));
      }).then(function (txid) {
        var form = document.querySelector(".claim");
        if (form && txid) {
          form.querySelector("[name=txid]").value = typeof txid === "string" ? txid : (txid.txid || txid);
          form.submit();
        }
      }).catch(function (e) {
        alert(e && e.message ? e.message : e);
      });
    };
  }
  if (cfg.id && cfg.status !== "Settled") {
    setInterval(function () {
      fetch("/api/v1/invoices/" + cfg.id).then(function (r) { return r.json(); }).then(function (inv) {
        if (inv && inv.status === "Settled") location.href = "/receipt/" + cfg.id;
        if (inv && inv.status === "Processing" && cfg.status === "New") location.reload();
      }).catch(function () {});
    }, 2000);
  }
})();
