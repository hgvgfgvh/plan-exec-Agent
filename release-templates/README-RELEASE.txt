AgentTest Windows 发布包 — 使用说明
================================

版本见 VERSION.txt

系统要求
--------
- Windows 10/11 x64
- 可访问互联网（DeepSeek API；可选阿里云 DashScope）
- Microsoft Edge WebView2 运行时（多数 Win11 已自带）
- 无需安装 Go、.NET SDK 或 Node.js

快速开始
--------
1. 解压本文件夹到任意路径（路径尽量不要含特殊字符）
2. 双击 Start-AgentTest-Cat.bat（或 AgentTestCat.exe）
3. 首次运行填写 DeepSeek API Key（必填）与可选 DashScope Key
4. 保存后自动启动内核、打开浏览器 WebUI、显示桌宠

目录说明
--------
AgentTest.exe          Go 内核（小猫会自动拉起）
AgentTestCat.exe       桌面小猫（推荐入口）
config/app.example.yaml  配置模板；首次配置后生成 config/app.yaml
WorkSpace/mcp_bundled/   内置 MCP
WorkSpace/               运行时日志与产物

仅启动内核（高级）
------------------
  copy config\app.example.yaml config\app.yaml
  编辑 config\app.yaml 填入 API Key
  AgentTest.exe
  浏览器 http://127.0.0.1:8765

安全提示
--------
- 勿将 WebUI 暴露到公网
- config/app.yaml 含密钥，勿分享
- 工具可执行本机 PowerShell / 文件 / 邮件 / 浏览器操作

开源仓库
--------
https://github.com/hgvgfgvh/plan-exec-Agent
