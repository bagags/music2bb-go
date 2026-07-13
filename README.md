# music-to-bb

酷狗音乐歌单 → Bilibili 收藏夹自动转换工具。

输入酷狗概念版歌单链接，自动解析歌曲列表、搜索 Bilibili 视频、智能匹配并一键添加到你的 B 站收藏夹。

## 功能

- **歌单解析** — 支持 HTTP API 和 Playwright 浏览器双引擎，兼容各种酷狗分享链接
- **智能匹配** — 多维评分算法（关键词、音质标签、官方认证、播放量/收藏量）自动匹配最佳视频
- **双模式** — CLI 命令行 + GUI 图形界面，按需选择
- **扫码登录** — Bilibili 二维码登录，Cookie 本地持久化
- **手动审核** — 支持自动匹配后逐首审核，或完全手动选择视频（支持 BV 号直接输入）
- **可定制** — 屏蔽词(b.txt)、加权词(w.txt)、UP主加权(w-up.txt) 均可自行编辑

## 安装

```bash
# 克隆仓库
git clone https://github.com/gguage/music-to-bb.git
cd music-to-bb

# 安装依赖
pip install -r requirements.txt

# 安装 Playwright 浏览器（GUI/浏览器解析模式需要）
playwright install chromium
```

## 使用

### GUI 模式

```bash
python main.py gui
```

### CLI 模式

```bash
# 基本用法
python main.py cli "https://m.kugou.com/share/zlist.html?id=xxx"

# 增加搜索页数提高匹配率
python main.py cli "链接" --search-pages 5

# 自动匹配后逐首审核
python main.py cli "链接" --manual-review

# 完全手动匹配
python main.py cli "链接" --manual
```

## 配置文件

| 文件 | 作用 |
|------|------|
| `b.txt` | 屏蔽关键词（翻唱、伴奏、cover、教程等），匹配到则跳过该视频 |
| `w.txt` | 加权关键词（官方、MV、无损、4K、Hi-Res等），匹配到则加分 |
| `w-up.txt` | UP主加权列表，指定UP主的视频优先匹配 |

## 项目结构

```
music-to-bb/
├── main.py          # 入口：CLI 参数解析
├── core.py          # 核心流程：解析→匹配→收藏
├── kugou.py         # 酷狗歌单爬取（HTTP API + Playwright）
├── bilibili.py      # Bilibili API 客户端（搜索/收藏/登录）
├── matcher.py       # 视频匹配评分引擎
├── models.py        # 数据模型
├── gui.py           # GUI 界面（CustomTkinter）
├── manual_match.py  # 交互式手动匹配
├── b.txt            # 屏蔽关键词
├── w.txt            # 加权关键词
├── w-up.txt         # UP主加权列表
└── requirements.txt
```
