# gitcn

面向国内网络的 GitHub 访问加速工具：一条命令自动选择最优代理节点、透明改写 URL、失败自动切换。

## 安装

需要 Go ≥ 1.22：

```bash
go install github.com/caiqianzhang/gitcn/cmd/gitcn@latest   # 发布后可用
# 或本地编译：
go build -o ~/.local/bin/gitcn ./cmd/gitcn
```

（个人工具，不提供自动发布/自更新。）

## 用法

```bash
gitcn clone octocat/Hello-World          # 加速克隆（支持 owner/repo 简写）
gitcn pull                               # 加速拉取（当前仓库）
gitcn fetch                              # 加速拉取（不合并）
gitcn download https://github.com/foo/bar/releases/download/v1.0/app.zip
gitcn download https://github.com/foo/bar/releases/download/v1.0/app.zip --sha256 <64位hex>   # 下载后校验 SHA-256，不匹配删文件并切换节点
gitcn nodes list                         # 查看节点与延迟
gitcn nodes update                       # 从远端 JSON 刷新节点
gitcn nodes add my.node.example          # 手动添加节点
gitcn config cache_ttl 30m               # 修改测速缓存时长（默认 15m，节点变化大可调短）
gitcn config timeout 10s                 # 修改测速/请求超时（默认 5s，慢节点可调大）
gitcn config download_timeout 20m        # 修改下载超时（默认 10m，大文件可调大）
gitcn config test_file_raw_url <url>     # 自定义测速探测文件（默认本仓库 README.md）
```

`gitcn -v` / `gitcn version` 显示版本；查看候选节点与延迟用 `gitcn nodes list`，命令内部细节可用 `gitcn config verbose true` 打开。

非 GitHub 链接（如 GitLab）与 SSH 地址（`git@github.com:user/repo.git`）不做代理改写，直连执行；`push` 也始终直连。手动添加的节点必须是裸域名或 IP。

## 工作原理

1. 若配置了镜像源，每次执行时按顺序拉取节点 JSON（`nodes.json`）；失败或未配置则用本地缓存，再失败用内置 83 节点种子
2. 缓存未过期直接用上次最优节点；过期则并发延迟测速（探测本仓库 README.md，识别假代理），取前 3 名候选
3. 把 GitHub URL 改写为 `https://节点/原URL` 后调用系统 git（clone/fetch/pull 均可，push 不包装、始终直连）
4. 失败自动切换下一个候选节点；全部失败提示直连

失败切换时会把失败节点标记为 dead 并写回本地缓存，短时间内不再优先选择它。

## 节点列表更新（可选）

默认使用内置 83 节点种子，种子会随时间失效。如需自动更新，把 `nodes.json` 发布到任意静态托管（如 Gitee/Cloudflare Pages），然后：

```bash
gitcn config mirror_sources https://你的地址/nodes.json
gitcn nodes update
```

`nodes.json` 格式：

```json
{
  "version": 1,
  "updated_at": "2026-08-19T00:00:00+08:00",
  "nodes": ["gh.dpik.top", "gh.sixyin.com"]
}
```

也可用 `gitcn nodes add <域名>` 临时添加节点。节点为社区志愿者提供的免费代理，**仅供学习研究**，使用风险自负。

## 免责声明

本项目与 GitHub, Inc. / Microsoft 无任何隶属关系；请遵守 GitHub 服务条款与所在地法律。
