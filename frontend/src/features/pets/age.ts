export function todayUTC() { return new Date().toISOString().slice(0, 10) }

export function ageAt(birthDate: string, approximate: boolean, date = todayUTC()) {
  if (!birthDate || date < birthDate) return { ageLabel: '', ageMonths: -1 }
  const birth = new Date(`${birthDate}T00:00:00Z`)
  const now = new Date(`${date}T00:00:00Z`)
  if (!Number.isFinite(birth.getTime()) || !Number.isFinite(now.getTime())) return { ageLabel: '', ageMonths: -1 }
  const months = (now.getUTCFullYear() - birth.getUTCFullYear()) * 12 + now.getUTCMonth() - birth.getUTCMonth() - Number(now.getUTCDate() < birth.getUTCDate())
  const ageLabel = months >= 12 ? `${Math.floor(months / 12)}岁${months % 12 ? `${months % 12}个月` : ''}`
    : months > 0 ? `${months}个月` : `${Math.floor((now.getTime() - birth.getTime()) / 86400000)}天`
  return { ageLabel: `${approximate ? '约' : ''}${ageLabel}`, ageMonths: Math.max(0, months) }
}

/** An estimated birthday is always marked approximate; clamp month-end dates. */
export function estimatedBirthday(months: number, date = todayUTC()) {
  const now = new Date(`${date}T00:00:00Z`)
  const target = new Date(Date.UTC(now.getUTCFullYear(), now.getUTCMonth() - months, 1))
  const last = new Date(Date.UTC(target.getUTCFullYear(), target.getUTCMonth() + 1, 0)).getUTCDate()
  target.setUTCDate(Math.min(now.getUTCDate(), last))
  return target.toISOString().slice(0, 10)
}
