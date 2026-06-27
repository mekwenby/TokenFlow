(function () {
  var stored = localStorage.getItem("theme");
  if (stored === "dark" || stored === "light") {
    document.documentElement.setAttribute("data-theme", stored);
  }
}());
