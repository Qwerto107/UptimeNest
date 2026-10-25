# 本地 UI 组件

按 [shadcn-vue](https://www.shadcn-vue.com/) 的源码组件模式维护，使用 Tailwind CSS、class-variance-authority 与 Reka UI。由于当前网络无法访问组件注册表，这些基础组件按其公开组件规范本地整理，并加入数字输入与中文无障碍标签支持。

对话框与确认框使用 Reka UI 的 Dialog 和 AlertDialog，负责焦点管理、键盘交互和无障碍语义。

下拉选择使用本地 Select 组件，复用 Reka UI 的焦点管理、键盘选择和表单语义；触发器与选项列表随项目主题显示，支持分组选项以及字符串或数字值。
