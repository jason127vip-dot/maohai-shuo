import { useId } from 'react'
import type { ArtStyleId, SceneId } from '../features/generation/types'

/** Original SVG illustrations for UI examples; not examples of actual model output. */
export function PetArt({ scene = 'mischief', style = 'cartoon', className = '' }: {
  scene?: SceneId; style?: ArtStyleId; className?: string
}) {
  const id = useId().replace(/:/g, '')
  const sketch = style === 'sketch'
  const watercolor = style === 'watercolor'
  const fur = sketch ? '#eeeae1' : watercolor ? '#dca778' : '#eeb16e'
  const line = sketch ? '#68625b' : '#674934'
  return (
    <svg className={className} viewBox="0 0 300 250" fill="none" aria-hidden="true">
      <defs>
        <pattern id={`${id}grain`} width="7" height="7" patternUnits="userSpaceOnUse"><circle cx="2" cy="3" r="0.7" fill={line} opacity="0.1" /></pattern>
        <filter id={`${id}wash`} x="-20%" y="-20%" width="140%" height="140%"><feGaussianBlur stdDeviation="0.8" /></filter>
      </defs>
      {watercolor && <g opacity="0.25" filter={`url(#${id}wash)`}><ellipse cx="150" cy="145" rx="104" ry="83" fill="#e4b59b" /><ellipse cx="120" cy="126" rx="90" ry="70" fill="#c8d8c4" /></g>}
      <ellipse cx="155" cy="219" rx="93" ry="9" fill={sketch ? '#dedbd2' : '#b9a083'} opacity="0.17" />
      {scene === 'door' && <g stroke={line} strokeWidth="3" opacity="0.3"><path d="M68 210V33h164v177M84 204V49h132v155"/><circle cx="199" cy="131" r="3" fill={line}/></g>}
      {scene === 'sleep' && <g transform="translate(7 19)"><rect x="64" y="172" width="177" height="35" rx="7" fill={sketch ? '#c9c5bc' : '#b7c6b8'} stroke={line} strokeWidth="2.5"/><path d="M78 181h149M78 190h149M92 175v22m18-22v22m18-22v22m18-22v22m18-22v22m18-22v22m18-22v22" stroke={line} strokeWidth="1.5" opacity="0.3"/></g>}
      <g stroke={line} strokeWidth={sketch ? 2.3 : 3.5} strokeLinecap="round" strokeLinejoin="round" filter={watercolor ? `url(#${id}wash)` : undefined}>
        <path d="M205 185c48 20 62-27 35-36-20-5-27 18-12 22" stroke={fur} strokeWidth="18"/>
        <path d="M107 139c-11 17-18 44-11 66 10 15 91 16 108-1 10-20 0-49-14-68" fill={fur}/>
        <ellipse cx="153" cy="181" rx="30" ry="34" fill={sketch ? '#faf8f1' : '#fff4db'} stroke="none"/>
        <path d="m96 85 1-51 41 29m27-1 39-31 4 57" fill={fur}/>
        <path d="m106 67 1-19 18 17m52 1 18-17 4 23" fill={sketch ? '#d8d1c9' : '#e58f7c'} stroke="none"/>
        <path d="M211 105c0 36-24 55-59 55s-62-18-62-52c0-28 21-49 61-49 37 0 60 19 60 46Z" fill={fur}/>
        <path d="m141 63 6 19m14-20-3 20m18-15-9 18" stroke={sketch ? '#9c9488' : '#c98143'} strokeWidth="7"/>
        <path d="m98 104 13 4m-12 10 11-1m94-14-13 4m12 11-12-1" stroke={sketch ? '#aaa194' : '#c98143'} strokeWidth="5"/>
        <ellipse cx="151" cy="134" rx="23" ry="17" fill={sketch ? '#faf8f1' : '#fff4db'} stroke="none"/>
        {scene === 'sleep' ? <><path d="m116 110 9 5 10-5m35 0 9 5 10-5"/><ellipse cx="153" cy="142" rx="6" ry="8" fill="#ac6e5e"/></> : <>
          <ellipse cx="124" cy="112" rx={scene === 'snack' ? 8 : 6} ry="10" fill={line} stroke="none"/>
          <ellipse cx="179" cy="112" rx={scene === 'snack' ? 8 : 6} ry="10" fill={line} stroke="none"/>
          <circle cx="126" cy="109" r="2" fill="white" stroke="none"/><circle cx="181" cy="109" r="2" fill="white" stroke="none"/>
          <path d="M141 138q5 7 11 0 6 7 11 0"/>
        </>}
        <path d="m148 129 4 3 4-3" fill={sketch ? '#9c837d' : '#cc8b7e'} strokeWidth="2.5"/>
        <path d="m93 132 22 2m-19 10 19-5m94-8-22 3m19 10-19-5" strokeWidth="1.5"/>
        <path d="M116 173v31q-14 13-22 3m88-34v31q14 13 22 3" fill={fur}/>
        <path d="m103 205 1 6m7-6v7m82-7v7m7-8 1 7" strokeWidth="2"/>
      </g>
      {scene === 'mischief' && <g stroke={line} strokeWidth="2" strokeLinejoin="round"><path d="m47 178 26-9 14 42-27 9Z" fill={sketch ? '#ddd8cf' : '#a0b5a4'}/><path d="m42 175 31-10 4 8-31 10Z" fill={sketch ? '#c7c3bb' : '#829d87'}/><path d="m39 218 16-9 7 10-18 7m40-4 13-3 8 9-17 1m128-9 11-9 15 7-13 10" fill="#fffdf5"/><path d="m64 128 4-8m-21 28-10-1m193-59 6-7" stroke="#cda06c" strokeWidth="3"/></g>}
      {scene === 'snack' && <g stroke={line} strokeWidth="2.5"><path d="m213 68 33-20 11 13-34 22Z" fill={sketch ? '#dad3c4' : '#d69454'}/><path d="m226 61 10 13m-1-19 10 13"/><path d="M190 129q11 17 2 21" stroke="#87b9d3" strokeWidth="5" strokeLinecap="round"/><path d="m76 74 3-9m-18 22-10-2" stroke="#d6a35d" strokeWidth="3"/></g>}
      {scene === 'sleep' && <g fill={line} opacity="0.6" fontFamily="sans-serif" fontWeight="700"><text x="217" y="78" fontSize="26" transform="rotate(12 217 78)">z</text><text x="244" y="48" fontSize="34" transform="rotate(12 244 48)">z</text></g>}
      {scene === 'door' && <path d="M56 95c-13-15-28 5 0 22 28-17 13-37 0-22Z" fill="#dba59b" opacity="0.8"/>}
      {sketch && <rect width="300" height="250" fill={`url(#${id}grain)`} />}
    </svg>
  )
}
