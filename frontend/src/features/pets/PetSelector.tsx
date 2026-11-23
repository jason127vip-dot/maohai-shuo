import { useEffect, useLayoutEffect, useState } from 'react'
import { Link } from 'react-router'
import { PawPrint } from 'lucide-react'
import { usePets } from './usePets'
import { petAvatars } from './types'
import './pets.css'

export function PetSelector({ value, showLabel, disabled, onChange, onLabelChange, onReadyChange }: { value: string | null; showLabel: boolean; disabled: boolean; onChange(id: string): void; onLabelChange(value: boolean): void; onReadyChange(ready: boolean): void }) {
  const { pets, loading, error, reload } = usePets()
  const [continueWithoutPet, setContinueWithoutPet] = useState(false)
  useEffect(() => { if (!disabled && value === null && !loading && !error && pets.length > 0) onChange(pets.find(pet => pet.isDefault)?.id || pets[0].id) }, [value, pets, loading, error, disabled, onChange])
  const selected = pets.find(pet => pet.id === value)
  const ready = !loading && (error ? continueWithoutPet && value === '' : value === '' || Boolean(selected) || value === null && pets.length === 0)
  useLayoutEffect(() => { onReadyChange(ready) }, [ready, onReadyChange])
  return <section className="pet-selector" aria-label="本次创作的宠物">
    <div className="pet-selector-title"><strong><PawPrint size={15} /> 为哪只毛孩记录？</strong><Link to="/pets">{pets.length ? '管理宠物' : '添加宠物'}</Link></div>
    {loading ? <p className="pet-field-hint" role="status">正在读取宠物资料…</p> : error ? <><div className="inline-error" role="alert"><span>{error}</span><button type="button" disabled={disabled} onClick={() => { setContinueWithoutPet(false); reload() }}>重新读取</button></div>{continueWithoutPet ? <p className="pet-field-hint" role="status">本次创作暂不关联宠物。</p> : <button className="pet-continue" type="button" disabled={disabled} onClick={() => { onChange(''); setContinueWithoutPet(true) }}>暂不关联宠物，继续制作</button>}</> : <>
      <label className="visually-hidden" htmlFor="creation-pet">选择本次创作的宠物</label>
      <select id="creation-pet" value={value || ''} disabled={disabled} onChange={event => onChange(event.target.value)}>
        <option value="">暂不关联宠物</option>{value && !selected && <option value={value}>资料已变更，请重新选择宠物</option>}
        {pets.map(pet => <option key={pet.id} value={pet.id}>{petAvatars[pet.avatar]} {pet.name}{pet.ageLabel ? ` · ${pet.ageLabel}` : ''}{pet.isDefault ? ' · 默认' : ''}</option>)}
      </select>
      {selected ? <label className="pet-checkbox"><input type="checkbox" checked={showLabel} disabled={disabled} onChange={event => onLabelChange(event.target.checked)} />在图片上显示名字{selected.ageLabel ? '和年龄' : ''}</label> : <p className="pet-field-hint">设置宠物资料后，作品可以自动进入它的成长轨迹。</p>}
    </>}
  </section>
}
