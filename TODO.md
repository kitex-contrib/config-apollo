
1. 文档里未明确说明应如何配置
2. server 启动时直接 panic「key not found」。用户没有配置是常态，不能因为用户没配置就直接崩溃。
3. 被注释的代码应直接删除(如 apollo.go 里的 RegisterConfigCallback)
4. 新启动的 goroutine 需要用 recover 做好保护，避免 panic 导致进程整体崩溃
5. 删除配置(如timeout)的时候没有生效(从NewValue里取不到数据就直接返回了)
6. retryContainer 的初始化需要优化，参考 https://github.com/kitex-contrib/config-nacos/pull/7/files#diff-61bf3033ef78457e64a31ad9b2b06f66ca32af5ac93dadee9856c5084aaa42d9R54，换成  NewRetryContainerWithPercentageLimit，并需要注册 closeCallback 回收资源
7. 默认值和文档要保持一致
8. apollo.New(..) 改名为 apollo.NewClient(..) 含义更明确
9. 需要补充用户文档
