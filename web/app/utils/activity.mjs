/** @param {Record<string, unknown>} query */
export function activityQuery(query) {
  const { limit: _limit, ...filters } = query
  return filters
}

/** @param {{ count: number }[]} buckets */
export function activityBars(buckets) {
  const maximum = Math.max(1, ...buckets.map((bucket) => bucket.count))
  const step = 600 / Math.max(1, buckets.length)

  return buckets.map((bucket, index) => {
    const height = (bucket.count / maximum) * 160

    return {
      x: index * step + step * 0.16,
      y: 170 - height,
      width: step * 0.68,
      height,
    }
  })
}

/** @param {{ count: number }[]} buckets */
export function activityLine(buckets) {
  const bars = activityBars(buckets)
  if (!bars.length) {
    return ''
  }

  if (bars.length === 1) {
    return `0,${bars[0].y} 600,${bars[0].y}`
  }

  return bars.map((bar) => `${bar.x + bar.width / 2},${bar.y}`).join(' ')
}

/** @param {{ total: number; breakdown: { key: string; count: number }[] }} activity */
export function activitySuccessRate(activity) {
  if (!activity.total) {
    return null
  }

  const successful = activity.breakdown.find((item) => item.key === 'success')?.count ?? 0
  return ((successful / activity.total) * 100).toFixed(1)
}
