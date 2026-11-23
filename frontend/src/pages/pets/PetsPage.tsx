import { useState } from 'react'
import { ArrowLeft, ArrowRight, PawPrint, Plus, Pencil } from 'lucide-react'
import { Link } from 'react-router'
import { Button } from '../../components/Button'
import { PetEditor } from '../../features/pets/PetEditor'
import { usePets } from '../../features/pets/usePets'
import { petAvatars, type Pet } from '../../features/pets/types'
import './PetsPage.css'

export function PetsPage() {
  const { pets, error, loading, reload } = usePets()
  const [editing, setEditing] = useState<Pet | 'new' | null>(null)
  return <main id="main-content" className="pets-page">
    <Link className="back-link" to="/"><ArrowLeft size={16} /> 返回制作页</Link>
    <div className="pets-heading"><div><span className="eyebrow"><span /> 每只毛孩，都值得被记住</span><h1>我的宠物</h1><p>记下它的名字，让每一次创作都有归属。</p></div><Button onClick={() => setEditing('new')}><Plus size={17} /> 添加宠物</Button></div>
    {loading ? <p className="history-status" role="status">正在读取宠物资料…</p> : error ? <div className="history-error" role="alert"><p>{error}</p><Button variant="secondary" onClick={reload}>重新读取</Button></div>
      : pets.length ? <div className="pets-grid">{pets.map(pet => <article className="pet-card" key={pet.id}>
        <span className="pet-card-avatar" role="img" aria-label="宠物头像">{petAvatars[pet.avatar]}</span>
        <div className="pet-card-title"><h2>{pet.name}</h2>{pet.isDefault && <span>默认宠物</span>}</div>
        <p className="pet-card-age">{pet.ageLabel || '年龄未设置'}{pet.species ? ` · ${pet.species === 'cat' ? '猫咪' : '狗狗'}` : ''}</p>
        {pet.birthDate && <p className="pet-card-birthday">{pet.approximate ? '估计生日' : '生日'}：{pet.birthDate}</p>}
        <div className="pet-card-actions"><button type="button" onClick={() => setEditing(pet)}><Pencil size={14} /> 编辑资料</button><Link to={`/growth?pet=${encodeURIComponent(pet.id)}`}>成长轨迹 <ArrowRight size={14} /></Link></div>
      </article>)}</div> : <section className="pets-empty"><PawPrint size={42} /><h2>先认识一下你的毛孩</h2><p>添加名字和年龄，以后的作品就能串成它的成长故事。</p><Button onClick={() => setEditing('new')}>添加第一只宠物</Button></section>}
    <Link className="text-link pets-growth-link" to="/growth">查看成长轨迹 <ArrowRight size={15} /></Link>
    {editing && <PetEditor pet={editing === 'new' ? undefined : editing} onClose={() => setEditing(null)} onSaved={() => { setEditing(null); reload() }} />}
  </main>
}
