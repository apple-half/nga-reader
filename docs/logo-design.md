# 应用图标

以下描述主分支源码的图标选择功能；0.1.13正式下载包仍使用此前的工位摸鱼图标，尚未发布包含本功能的新版本。

## 简约工作

默认规范文件为`packaging/icon.png`：深石墨色圆角方形底，米白色四段折带组成抽象工作空间标志，无文字、人物或论坛名称，外沿透明。它采用办公软件常见的克制黑白风格，图形为原创设计，不是ChatGPT或OpenAI官方标志。

使用内置图像生成工具生成；原始输出`exec-53029a6a-0e0e-44e8-bf4a-6770e00eedc8.png`已复制到规范源文件。最终提示词核心：professional workspace productivity application icon, original four broad rounded interlocking folded ribbon strokes, compact square-knot / abstract workspace aperture, off-white geometric mark on nearly-black graphite rounded-square tile, transparent outside, simple enough at16px, no text, no NGA, no person, no fish, no existing ChatGPT/OpenAI knot or six-lobed flower.

Windows ICO及Mac ICNS从规范文件打包，因此程序文件及快捷方式始终使用简约工作图标。设置可即时切换应用内、运行窗口、托盘和Mac Dock的图标，选择持久保存在本机设置；演示模式只临时预览。软件不改写EXE、应用包、快捷方式或系统图标缓存，避免破坏签名和便携升级。

## 工位摸鱼

按用户指定的角色设计：戴黑框眼镜、格子衫、发福的中年男性，在工位偷偷浏览NGA，卡通风格。深棕圆角底、暖色人物和奶油色论坛屏幕，外沿透明。人物不是鱼，也不是NGA官方吉祥物。

规范源文件保留为`packaging/icon-office.png`，与0.1.13发布时的图标逐字节相同，使用内置图像生成工具生成并按用户修正迭代。在设置中选择“工位摸鱼”即可使用。

最终提示词核心：chubby middle-aged human office worker, black rectangular glasses, thinning black hair, red-brown and cream plaid shirt, amused slightly guilty expression, seated at desk secretly browsing NGA, large readable NGA on monitor, clean polished 2D cartoon, warm espresso rounded tile, transparent outside, no fish or other characters.

发布前检查PNG尺寸、透明外沿、源文件一致性和16/32/64/128px原生预览。16px仅保留整体人物／屏幕轮廓，细节以较大系统图标为准。平台ICO/ICNS由MyGo从规范PNG生成。
