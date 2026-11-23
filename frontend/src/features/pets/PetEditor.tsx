import { useState } from 'react'
import { Button } from '../../components/Button'
import { Modal } from '../../components/Modal'
import { petsApi } from './api'
import { estimatedBirthday, todayUTC } from './age'
import { petAvatars, type Pet, type PetInput } from './types'
import './pets.css'

export function PetEditor({ pet, onClose, onSaved }: { pet?: Pet; onClose(): void; onSaved(pet: Pet): void }) {
  const [name, setName] = useState(pet?.name || '')
  const [species, setSpecies] = useState<PetInput['species']>(pet?.species || '')
  const [avatar, setAvatar] = useState<PetInput['avatar']>(pet?.avatar || 'paw')
  const [birthDate, setBirthDate] = useState(pet?.birthDate || '')
  const [approximate, setApproximate] = useState(pet?.approximate || false)
  const initialMonths = Math.max(0, pet?.ageMonths ?? 0)
  const [years, setYears] = useState(String(Math.floor(initialMonths / 12)))
  const [months, setMonths] = useState(String(initialMonths % 12))
  const [isDefault, setDefault] = useState(pet?.isDefault || false)
  const [saving, setSaving] = useState(false)
  const [error, setError] = useState('')

  async function save(event: React.FormEvent<HTMLFormElement>) {
    event.preventDefault()
    if (saving) return
    const cleanName = name.trim()
    if (!cleanName || [...cleanName].length > 20 || [...cleanName].some(char => char.charCodeAt(0) < 32 || char.charCodeAt(0) === 127)) { setError('请填写1至20个字的宠物名字。'); return }
    const ageMonths = Number(years) * 12 + Number(months)
    if (approximate && (years === '' || months === '' || !Number.isInteger(Number(years)) || Number(years) < 0 || Number(years) > 100 || !Number.isInteger(Number(months)) || Number(months) < 0 || Number(months) > 11)) { setError('请填写有效年龄：0至100岁，月份为0至11。'); return }
    const birthday = approximate ? pet?.approximate && ageMonths === initialMonths ? pet.birthDate : estimatedBirthday(ageMonths) : birthDate
    setSaving(true); setError('')
    try {
      onSaved(await petsApi.save({ name: cleanName, species, avatar, birthDate: birthday, approximate, isDefault }, pet?.id))
    } catch (reason) { setError(reason instanceof Error ? reason.message : '保存失败，请稍后重试。') }
    finally { setSaving(false) }
  }

  return <Modal title={pet ? '编辑宠物资料' : '认识一下你的毛孩'} onClose={() => { if (!saving) onClose() }}>
    <form className="pet-editor" onSubmit={event => { void save(event) }}>
      <fieldset disabled={saving}>
        <label>宠物名字 <span className="pet-required">必填</span><input name="pet-name" value={name} maxLength={40} required placeholder="比如：奶茶" onChange={event => setName(event.target.value)} /></label>
        <label>它是<select name="pet-species" value={species} onChange={event => setSpecies(event.target.value as PetInput['species'])}><option value="">暂不填写</option><option value="cat">猫咪</option><option value="dog">狗狗</option></select></label>
        <fieldset className="pet-avatar-options"><legend>选一个头像图标</legend>{(Object.entries(petAvatars) as [PetInput['avatar'], string][]).map(([id, emoji]) => <label key={id} className={avatar === id ? 'is-selected' : ''}><input type="radio" name="pet-avatar" value={id} checked={avatar === id} onChange={() => setAvatar(id)} /><span role="img" aria-label={id === 'cat' ? '猫咪' : id === 'dog' ? '狗狗' : '爪印'}>{emoji}</span></label>)}</fieldset>
        <label className="pet-checkbox"><input type="checkbox" checked={approximate} onChange={event => setApproximate(event.target.checked)} />不确定生日，填写大概年龄</label>
        {approximate ? <div className="pet-age-inputs"><label>岁<input name="pet-years" type="number" min="0" max="100" step="1" required value={years} onChange={event => setYears(event.target.value)} /></label><label>个月<input name="pet-months" type="number" min="0" max="11" step="1" required value={months} onChange={event => setMonths(event.target.value)} /></label></div>
          : <label>生日 <span className="pet-optional">可选</span><input name="pet-birthday" type="date" min="1900-01-01" max={todayUTC()} value={birthDate} onChange={event => setBirthDate(event.target.value)} /></label>}
        <p className="pet-field-hint">{approximate ? '年龄会标注“约”，以后也会随时间增长。' : '填写生日后，会自动计算每次创作时的年龄。'}</p>
        <label className="pet-checkbox"><input type="checkbox" checked={isDefault} disabled={pet?.isDefault} onChange={event => setDefault(event.target.checked)} />设为默认宠物</label>
      </fieldset>
      {error && <p className="error-message" role="alert">{error}</p>}
      <Button type="submit" busy={saving}>保存宠物资料</Button>
    </form>
  </Modal>
}
