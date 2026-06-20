// Human-readable byte sizes. Ported from the old prettyBytes Vue filter
// (Vue 3 removed filters), originally adapted from sindresorhus/pretty-bytes.
export function prettyBytes(num) {
  if (typeof num !== 'number' || isNaN(num)) {
    return '0 B'
  }

  const units = ['B', 'kB', 'MB', 'GB', 'TB', 'PB', 'EB', 'ZB', 'YB']
  const neg = num < 0
  if (neg) {
    num = -num
  }
  if (num < 1) {
    return (neg ? '-' : '') + num + ' B'
  }

  const exponent = Math.min(
    Math.floor(Math.log(num) / Math.log(1024)),
    units.length - 1
  )
  num = (num / Math.pow(1024, exponent)).toFixed(2) * 1

  return (neg ? '-' : '') + num + ' ' + units[exponent]
}
