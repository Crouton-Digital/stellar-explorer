export function short(value = '', size = 8) {
  return value.length <= size * 2 + 1 ? value : `${value.slice(0, size)}…${value.slice(-size)}`
}

export function xlm(stroops) {
  if (!/^-?\d+$/.test(String(stroops))) {
    return '—'
  }

  const n = BigInt(stroops)
  const abs = n < 0n ? -n : n

  return `${n < 0n ? '-' : ''}${abs / 10000000n}.${String(abs % 10000000n).padStart(7, '0')}`
}

export function apiError(error) {
  return error?.data?.error?.message || 'The explorer API is unavailable. Try again shortly.'
}

/**
 * @param {string} value
 * @param {number | null} [decimals]
 */
export function tokenAmount(value, decimals = null) {
  if (typeof value !== 'string' || !/^\d+$/.test(value)) {
    return '—'
  }

  const amount = BigInt(value)
  if (decimals === null || !Number.isInteger(decimals) || decimals < 0 || decimals > 255) {
    return amount.toString()
  }

  if (decimals === 0) {
    return amount.toString()
  }

  const scale = 10n ** BigInt(decimals)

  return `${amount / scale}.${String(amount % scale).padStart(decimals, '0')}`
}
