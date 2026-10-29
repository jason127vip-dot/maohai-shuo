import './generation.css'
import { Check } from 'lucide-react'
import { PetArt } from '../../../components/PetArt'
import { artStyles } from '../options'
import type { ArtStyleId, SceneId } from '../types'

export function StylePicker({ value, scene, onChange, disabled }: {
  value: ArtStyleId | null; scene: SceneId | null; onChange(value: ArtStyleId): void; disabled: boolean
}) {
  return (
    <div className="style-grid">
      {artStyles.map(style => (
        <label className={`style-option style-option--${style.id} ${value === style.id ? 'is-selected' : ''}`} key={style.id}>
          <input type="radio" name="style" value={style.id} checked={value === style.id} disabled={disabled} onChange={() => onChange(style.id)} />
          <div className="style-art"><PetArt style={style.id} scene={scene || 'mischief'} /></div>
          <div className="style-label"><strong>{style.title}</strong><span>{style.subtitle}</span></div>
          <span className="style-radio">{value === style.id && <Check size={11} strokeWidth={3} />}</span>
        </label>
      ))}
    </div>
  )
}
