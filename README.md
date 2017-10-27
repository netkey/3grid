# grid
A Global Route IP Director(Daemon), aka GSLB.

模块	功能	完成情况	预计时间

消息队列
(AMQP)	心跳（性能数据、健康数据）	√	
	版本查询（版本数据）	√	
	消息收发（封装、解封装、解压、）	√	
	基础消息队列功能（建连、广播、路由、反射处理）	√	
	核心数据增删改(增量更新)	√	
	业务状态（state）数据发送	√	
			

DNS	Worker循环	√	
	多DNS进程（reuse_port）	√	
	A/AAAA	√	
	NS	√	
	CNAME	√	
	TXT	√	
	SOA	√	
	DNSSEC/EDNS	√	
	DNS over TLS	√	
	DNS name compression	√	
	DNS查询log（文件和消息队列）	√	
	域名QPS状态	√	
			

IP库	全球范围IP段数据	√	
	中国范围IP段数据	√	
	DB并发查找	√	
	AC_IP Cache
并发读写	√	
	AC_IP Cache TTL	√	
	IP库动态版本更新	√	
			

路由表	主数据结构（域名、节点、服务器、路由方案、Cache、读写锁、任务Channel）	√	
	库文件加载，数据结构转换（域名、节点、服务器、路由方案）	√	
	数据结构更新维护线程	√	
	数据结构并发读写例程	√	
	AC_Domain Cache维护	√	
	AC_Domain Cache TTL	√	
	库文件版本更新后动态加载	√	
	路由父方案和缺省方案查找支持	√	
			

算法	状态数据结构（status、usage、weight、priority等）	√	
	AC_RR(路由方案)查找
（最长匹配算法）	√	
	AC_Domain_IP查找主例程
（GetAAA）	√	
	状态（usage、load）算法	√	
	优先级权重
（priority、weight）算法	√	
	切峰算法（优先切非本地访问）	√	
	切峰阀值可定义	√	
	成本算法	√	
	路由黑洞	0%	10.30
	直接回源	√	
	服务器A记录按照权重空闲%排列	√	
			

其它	GSLB主程序框架
（SMP、协程处理架构）	√	
	全局数据（QPS、Load）维护更新	√	
	Debug框架（log信息输出）	√	
	版本号分析和更新-公共例程	√	
	主库物理文件版本比较和下载更新
-辅助工具程序	√	
	IP库查询
-辅助工具程序	√	
	主程序配置文件(conf)加载/动态加载	√/√	
	miekg-dns-server-go.patch
(添加reuse_port支持)	√	
	GSLB master/worker(主控/工作)方式	√	
			
			
			
