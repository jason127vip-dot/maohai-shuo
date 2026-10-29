import type { ArtStyleId, SceneId } from './types'

export const scenes: { id: SceneId; title: string; subtitle: string; icon: 'house' | 'snack' | 'moon' | 'door' }[] = [
  { id: 'mischief', title: '拆家现场', subtitle: '不是我，真的', icon: 'house' },
  { id: 'snack', title: '在线讨吃', subtitle: '就一小口嘛', icon: 'snack' },
  { id: 'sleep', title: '奇妙睡姿', subtitle: '已进入梦乡', icon: 'moon' },
  { id: 'door', title: '门口等你', subtitle: '你怎么还不回来', icon: 'door' },
]

// Display labels only. Model-specific prompts and provider selection belong in Go.
export const artStyles: { id: ArtStyleId; title: string; subtitle: string }[] = [
  { id: 'sketch', title: '铅笔素描', subtitle: '随手画的小可爱' },
  { id: 'cartoon', title: '元气卡通', subtitle: '可爱浓度超标' },
  { id: 'watercolor', title: '温柔水彩', subtitle: '晕染一点小情绪' },
]
