# 工位摸鱼图标

按用户指定的角色设计：戴黑框眼镜、格子衫、发福的中年男性，在工位偷偷浏览NGA，卡通风格。深棕圆角底、暖色人物和奶油色论坛屏幕，外沿透明。人物不是鱼，也不是NGA官方吉祥物。

规范源文件为`packaging/icon.png`，使用内置图像生成工具生成并按用户修正迭代。窗口、托盘、界面及Windows/macOS打包均引用同一份PNG，不再分别手绘不同图标。

最终提示词核心：chubby middle-aged human office worker, black rectangular glasses, thinning black hair, red-brown and cream plaid shirt, amused slightly guilty expression, seated at desk secretly browsing NGA, large readable NGA on monitor, clean polished 2D cartoon, warm espresso rounded tile, transparent outside, no fish or other characters.

发布前检查PNG尺寸、透明外沿、源文件一致性和16/32/64/128px原生预览。16px仅保留整体人物／屏幕轮廓，细节以较大系统图标为准。平台ICO/ICNS由MyGo从规范PNG生成。
