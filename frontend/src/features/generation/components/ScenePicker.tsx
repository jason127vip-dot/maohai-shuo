import './generation.css'
import { Check, Cookie, DoorOpen, House, MoonStar } from 'lucide-react'
import { scenes } from '../options'
import type { SceneId } from '../types'

const icons = { house: House, snack: Cookie, moon: MoonStar, door: DoorOpen }

export function ScenePicker({ value, onChange, disabled }: {
  value: SceneId | null; onChange(value: SceneId): void; disabled: boolean
}) {
  return (
    <div className="scene-grid">
      {scenes.map(scene => {
        const Icon = icons[scene.icon]
        return (
          <label className={`scene-option scene-option--${scene.id} ${value === scene.id ? 'is-selected' : ''}`} key={scene.id}>
            <input type="radio" name="scene" value={scene.id} checked={value === scene.id} disabled={disabled} onChange={() => onChange(scene.id)} />
            <span className="scene-icon"><Icon size={24} strokeWidth={1.6} /></span>
            <strong>{scene.title}</strong><span className="scene-subtitle">{scene.subtitle}</span>
            {value === scene.id && <span className="selection-check"><Check size={11} strokeWidth={3} /></span>}
          </label>
        )
      })}
    </div>
  )
}
