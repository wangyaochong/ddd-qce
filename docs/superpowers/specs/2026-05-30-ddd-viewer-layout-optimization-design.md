# DDD Viewer 空间优化设计

**日期**: 2026-05-30
**状态**: 已批准

## 问题

DDD Viewer 前端在宽屏显示器上空间利用率低:
- `max-width: 1100px` 导致1920px屏幕浪费约40%宽度
- 垂直间距过大,需要频繁滚动
- JSON预览区 `max-height: 200px` 过小

## 方案

采用**流式扩展**方案,保持现有导航结构和交互方式,仅调整CSS尺寸参数。

## 改动清单

修改 `observability/templates/ddd_layout.html` 的 `<style>` 区块:

| CSS选择器 | 属性 | 当前值 | 新值 |
|-----------|------|--------|------|
| `.container` | max-width | 1100px | 1600px |
| `.container` | margin | 2rem auto | 1rem auto |
| `.json-preview` | max-height | 200px | 400px |
| `.card` | padding | 1.5rem | 1rem |
| `.card` | margin-bottom | 1.5rem | 0.8rem |
| `nav` | height | 56px | 48px |
| `nav` | padding | 0 2rem | 0 1rem |
| `h1` | margin-bottom | 1.5rem | 0.8rem |
| `.stats` | grid-template-columns min | 160px | 120px |

## 效果

- **宽度**: 1920px屏幕利用率从57%提升至83%
- **高度**: 减少约30%垂直空白,滚动次数减少
- **JSON预览**: 可显示更多内容,减少展开操作

## 页面适配

所有现有页面无需修改模板结构,CSS改动自动生效:
- Overview: 表格列宽自动扩展
- Commands/Queries/Events: JSON预览区扩大
- Domains: 类型卡片网格更宽
- Schema: 详情展示更清晰

## 实现步骤

1. 修改 `ddd_layout.html` 的CSS样式
2. 运行现有测试验证页面渲染正常
3. 在宽屏浏览器中手动验证效果

## 风险

低风险 - 仅CSS参数调整,不改变HTML结构或交互逻辑。