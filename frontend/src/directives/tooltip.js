// Lightweight replacement for the old jQuery/Bootstrap tooltip directive.
// Uses the native browser title attribute — no jQuery dependency.
function setTitle(el, binding) {
  if (binding.value) {
    el.setAttribute('title', binding.value)
  }
}

export default {
  mounted: setTitle,
  updated: setTitle,
}
