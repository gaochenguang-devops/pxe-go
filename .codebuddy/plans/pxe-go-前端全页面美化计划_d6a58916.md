---
name: pxe-go-前端全页面美化计划
overview: 以已完成的"服务配置页（工业运维控制台风格）"为设计基准，分 11 个步骤统一美化 pxe-go 全部前端页面（登录页、仪表盘、主机管理、主机资源、引导脚本、系统镜像、文件管理、操作日志、装机记录及全部弹窗），每步独立可验收并标注完成进度。
design:
  architecture:
    framework: html
  styleKeywords:
    - 工业运维控制台
    - 深色科技感
    - 等宽数据字体
    - 渐变光晕
    - 微交互动效
  fontSystem:
    fontFamily: JetBrains Mono, PingFang SC, Microsoft YaHei
    heading:
      size: 20px
      weight: 800
    subheading:
      size: 14px
      weight: 700
    body:
      size: 13px
      weight: 400
  colorSystem:
    primary:
      - "#3b82f6"
      - "#22d3ee"
      - "#0f1420"
    background:
      - "#0f1420"
      - "#1a2233"
      - "#202b40"
      - "#0d1117"
    text:
      - "#e6edf7"
      - "#8b9bb4"
    functional:
      - "#22c55e"
      - "#f59e0b"
      - "#ef4444"
todos:
  - id: global-shell
    content: 用 [skill:ui-ux-pro-max] 与 [skill:frontend-design] 美化全局框架：登录页、侧边栏、顶栏，确立统一设计基座
    status: completed
  - id: dashboard-page
    content: 美化仪表盘：统计卡/服务状态/配置概览/最近操作，复用 stat-card 与徽章体系
    status: completed
    dependencies:
      - global-shell
  - id: host-pages
    content: 重构主机管理与主机资源页：工具栏/分页/表格/详情弹窗去内联样式并统一卡片化
    status: completed
    dependencies:
      - dashboard-page
  - id: script-pages
    content: 美化引导脚本页：分段控件、KS/iPXE/部署三面板、镜像选择区与编辑弹窗
    status: completed
    dependencies:
      - host-pages
  - id: image-file-pages
    content: 美化系统镜像与文件管理页：上传区/进度条/镜像卡片/文件表格统一卡片化
    status: completed
    dependencies:
      - script-pages
  - id: log-install-pages
    content: 美化操作日志与装机记录页：筛选工具栏/表格徽章/统计卡与记录行对齐工业风格
    status: completed
    dependencies:
      - image-file-pages
  - id: final-polish
    content: 统一全部弹窗样式，修复 cfg-tag 截断遗留问题，清理 CSS 死代码，全量回归验收
    status: completed
    dependencies:
      - log-install-pages
---

## 用户需求

- 生成一份**分步骤美化所有前端页面**的实施计划，先计划、确认后再逐步执行
- 每完成一个步骤**标注一个**（打勾/验收），直到全部完成，形成"逐步交付、逐步确认"的节奏
- 前端设计参考 skills（ui-ux-pro-max / frontend-design 的设计方法论）

## 产品概述

PXE 装机管理系统的 Web 管理后台（纯原生 HTML/CSS/JS，经 go:embed 编译进二进制），包含登录页、仪表盘、服务配置、主机管理、主机资源、引导脚本、系统镜像、文件管理、操作日志、装机记录 10 个页面及 10 个弹窗。

## 核心目标

- 以已完成的"工业运维控制台"风格（服务配置页）为**全局设计语言**，统一美化其余所有页面与弹窗
- 保持全部现有功能与交互不变（JS 的 id 与 onclick 函数名不可改动）
- 每完成一个页面即向用户报告并等待确认，再进入下一步

## 分步美化范围

1. 全局框架：登录页、侧边栏、顶部栏（设计基座）
2. 仪表盘
3. 主机管理、主机资源
4. 引导脚本（KS / iPXE / 部署脚本）
5. 系统镜像、文件管理
6. 操作日志、装机记录
7. 全局弹窗统一 + 修复遗留问题（hero 区 cfg-tag 文字截断、清理 CSS 死代码）+ 全量验收

## 关键约束

- 不改变任何 JS 元素的 id 与 onclick 绑定，纯 HTML 结构 + CSS 层面美化
- 桌面运维场景为主，兼顾 1280px 以上常见分辨率
- 每次改动后需 node 校验 JS 语法 + `go build` + 浏览器预览验收

## 技术栈

- 前端：纯原生 HTML5 + CSS3 + 原生 JavaScript（无框架、无构建工具）
- 嵌入方式：`web/ui` 目录通过 `//go:embed` 编译进 Go 二进制，改动无需部署前端静态文件
- 样式：单一 `web/ui/css/style.css`（约 340 行，已内置 CSS 变量设计令牌与组件类），沿用现有 `--bg/--card/--accent/--accent2` 等令牌，必要时增量扩展新令牌，避免推翻既有体系

## 实现方式

以"**工业运维控制台**"为统一设计语言（服务配置页已验证为最高水准，作为视觉模板），按页面逐一推广：

- 表格密集页（主机/主机资源/日志）：内联 `style=""` 的工具栏、搜索框、分页条全部抽取为语义化 CSS 类（如 `.toolbar`、`.pager`、`.page-head`），表格套用 `.tbl-wrap` 风格，IP/时间等数据列加 `.mono` 等宽字体与徽章
- 卡片/数据页（仪表盘/装机记录）：复用 `.stat-card`、增强 `.svc-item`，区块标题统一为带图标的 `.card-head` 结构
- 上传页（镜像/文件）：上传区、进度条、说明文字统一卡片化，复用 `.cfg-card` 质感
- 弹窗：全部弹窗的裸 `label+input` 升级为 `.f-label`/`.f-input` 结构，保存按钮统一 `.btn-save`
- 全局框架：侧边栏导航项加 hover/active 微交互，登录页升级品牌感（渐变背景、logo、动效）
- 修复遗留：配置页 hero 区 `.cfg-tag` 文字溢出（加 `white-space:nowrap` 与 `flex-wrap` 约束）；删除 style.css 中不再引用的旧日志死代码（`.cfg-log-panel/.cfg-log-header/.cfg-log-list/.cfg-log-item` 等）

## 性能与可靠性

- 纯 CSS/HTML 改造，无运行时开销；仅新增少量过渡动画（hover、fade）
- 不触碰任何 JS 逻辑与后端接口，功能回归风险极低
- 每步验收标准：node 校验 JS 语法通过、`go build ./...` 通过、启动服务浏览器预览无布局错乱、现有功能按钮可用

## 目录结构（本次改动仅涉及前端两文件 + 无后端改动）

```
web/
├── ui/
│   ├── index.html          # [MODIFY] 所有页面/弹窗的 HTML 结构美化（id/onclick 不动）
│   ├── css/
│   │   └── style.css       # [MODIFY] 增量新增/覆盖组件样式，清理死代码
│   └── js/                 # [UNCHANGED] 10 个 JS 文件不改动（仅验证语法）
```

## 验收闭环

每完成一个 todo 步骤：更新该步骤为 completed 状态（即"标注一个"），向用户简述该页面改动点，并提示可浏览器预览；全部完成后做一次全量回归（`go test ./...` + 全页面预览清单）。

## 设计方向：工业运维控制台（全局统一）

沿用服务配置页已验证的设计语言，向全站推广，形成一致的"运维控制台"气质。

### 设计基调

- 深空蓝底色（#0f1420）+ 卡片分层（#1a2233/#202b40），蓝青（#3b82f6→#22d3ee）为主 accent，绿/琥珀/红作功能色
- 等宽字体（JetBrains Mono/Fira Code）用于 IP、路径、端口、时间戳等数据，营造终端/控制台观感；中文正文用 PingFang SC/微软雅黑
- 卡片统一内阴影 + 顶部细微高光 + hover 上浮；区块标题用编号徽章（01/02/03）或图标，副标题为等宽小字
- 表格：表头深色、大写、等宽标签；行 hover 高亮；状态用徽章（badge）而非裸文字
- 输入聚焦：青绿描边发光（复用 .f-input:focus）
- 页面切换保留现有 fade 动画；按钮/卡片 hover 微交互

### 各页面设计要点

1. 登录页：径向渐变背景 + 品牌徽标 + 发光卡片，登录按钮全宽渐变
2. 仪表盘：统计卡增强（大数字 + 图标角标）、服务状态卡、配置概览卡、最近操作用徽章化表格
3. 主机/主机资源：工具栏（搜索+批量操作）卡片化、分页条统一样式、状态与电源操作徽章化、node-info 预览终端化
4. 引导脚本：tab-bar 升级为分段控件，三个子面板统一卡片头，img-picker 保留并精致化
5. 系统镜像：页头说明区卡片化，image-card 保持并增强 hover 动效，上传弹窗 f-label/f-input 化
6. 文件管理：上传区卡片 + 进度条渐变，文件表格链接/复制按钮徽章化
7. 操作日志：筛选工具栏卡片化，类型徽章配色复用 op-badge 体系
8. 装机记录：统计卡与记录行对齐工业风格，LLDP 标签保留
9. 全部弹窗：统一 modal 头部（图标标题）、f-label/f-input 字段、btn-save 主操作

### 布局与响应

- 内容区最大宽度 1400px，卡片间距 14-20px
- 表格容器横向滚动兜底，操作列不换行

## Agent Extensions

### Skill

- **ui-ux-pro-max**
- 用途：为每个页面的美化提供 UI/UX 设计智能指导（布局、组件规格、状态设计、色彩/字体令牌），贯穿全部步骤
- 预期产出：各页面组件级的设计规格与实现建议，保证全站设计一致性
- **frontend-design**
- 用途：提供避免"通用 AI 风格"的差异化美学方向，用于确立全局"工业运维控制台"视觉语言与页面特色
- 预期产出：具有识别度、非模板化的视觉方案（配色、字体、质感、动效）

说明：执行时在具体步骤中引用对应 skill（如仪表盘步骤使用 [skill:ui-ux-pro-max] 设计统计卡规格），确保每个页面的美化都有设计依据。