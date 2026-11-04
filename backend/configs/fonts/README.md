# 中文角标字体

真实模型模式需要为 `storage.font_path` 配置一个允许商用的中文 TTF/OTF 字体，例如自行获取有授权的 Noto Sans SC 字体。

路径相对 `config.local.yaml` 所在目录，例如 `fonts/NotoSansSC-Regular.otf`。字体文件默认不提交 Git，许可证由使用者保留。配置时请确认字体确实包含“毛孩儿说”和“AI 生成”字形。

程序在启动时加载并检查角标字形，避免已经调用付费模型后才发现无法加角标。演示模式使用内置英文字体，图片明确标为 DEMO，不冒充 AI 成品。

## 本机使用的字体

本机已下载官方 Noto Sans SC Regular（OTF），配置为：

```yaml
font_path: "fonts/NotoSansSC-Regular.otf"
```

来源：[Noto 官方字体仓库](https://github.com/notofonts/noto-cjk/blob/main/Sans/SubsetOTF/SC/NotoSansSC-Regular.otf)。许可证保存在同目录 `NotoSansSC-LICENSE.txt`（SIL Open Font License 1.1）。字体文件保持原始内容与文件名。

字体和许可证当前都被 Git 忽略；在其他电脑或服务器部署时，需要准备这两个文件并保持配置路径一致。
