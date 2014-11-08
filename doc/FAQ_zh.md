###Codis 是什么?

Codis 是 Wandoujia Infrastructure Team 开发的一个分布式 Redis 服务, 用户可以看成是一个无限内存的 Redis 服务, 有动态扩/缩容的能力. 对偏存储型的业务更实用, 如果你需要 SUBPUB 之类的指令, Codis 是不支持的. 时刻记住 Codis 是一个分布式存储的项目. 对于海量的 key, value不太大( <= 1M ), 随着业务扩展缓存也要随之扩展的业务场景有特效.


###我的服务能直接迁移到 Codis 上吗?

分两种情况: 
 
1) 原来使用 twemproxy 的用户:
可以, 使用codis项目内的redis-port工具, 可以实时的同步 twemproxy 底下的 redis 数据到你的 codis 集群上. 搞定了以后, 只需要你修改一下你的代码, 将 twemproxy 的地址改成 codis 的地址就好了. 除此之外, 你什么事情都不用做.

2) 原来使用 Redis 的用户:
看情况, 如果你使用以下命令:   

KEYS, MOVE, OBJECT, RENAME, RENAMENX, SORT, SCAN, BITOP,MSETNX, BLPOP, BRPOP, BRPOPLPUSH, PSUBSCRIBE，PUBLISH, PUNSUBSCRIBE,  SUBSCRIBE,  UNSUBSCRIBE,  DISCARD, EXEC, MULTI,  UNWATCH,  WATCH, SCRIPT EXISTS, SCRIPT FLUSH, SCRIPT KILL, SCRIPT LOAD, AUTH, ECHO, SELECT, BGREWRITEAOF, BGSAVE, CLIENT KILL, CLIENT LIST, CONFIG GET, CONFIG SET, CONFIG RESETSTAT, DBSIZE, DEBUG OBJECT, DEBUG SEGFAULT, FLUSHALL, FLUSHDB, INFO, LASTSAVE, MONITOR, SAVE, SHUTDOWN, SLAVEOF, SLOWLOG, SYNC, TIME

是无法直接迁移到 Codis 上的. 你需要修改你的代码, 用其他的方式实现.

###服务迁移到 Codis 上有什么好处?

Redis获得动态扩容/缩容的能力. 不需要业务方担心 Redis 内存爆掉的问题. 也不用担心申请太大, 造成浪费. 业务方也不需要自己维护 Redis.

###我的代码里用了 KEYS 怎么办?

Codis 是会把 KEYS 指令屏蔽的, 即使你在使用 raw Redis, 我也不太建议使用这个命令, 因为在 Redis 里 KEYS 指令是 O(n) 复杂度的, 而且 Redis 是一个单线程的服务端程序, 当 Key 的数量一大, 会将整个主线程卡死, 所有的请求都无法响应. 所以我建议在业务中最好别用. Codis 同样屏蔽 SCAN 命令.

###Codis 可以当队列使用吗?

可以, Codis 还是支持 LPUSH LPOP这样的指令, 但是注意, 并不是说你用了 Codis 就有了一个分布式队列. 对于单个 key, 它的 value 还是会在一台机器上, 所以, 如果你的队列特别大, 而且整个业务就用了几个 key, 那么就几乎退化成了单个 redis 了, 所以, 如果你是希望有一个队列服务, 那么我建议你:

1. List key 里存放的是任务id, 用另一个key-value pair来存放具体任务信息
2. 使用 Pull 而不是 Push, 由消费者主动拉取数据, 而不是生产者推送数据.
3. 监控好你的消费者, 当队列过长的时候, 及时报警. 
4. 可以将一个大队列拆分成多个小的队列, 放在不同的key中

###Codis 支持 MSET, MGET吗?

支持, 但是对于有高性能要求的场景, 尽量不要使用. 当然, 如果觉得你的瓶颈是带宽的话, 那可以用. 在 Codis 内部还是会将 MSET 和 MGET 拆成一个个单个GET, SET指令的, 所以在Codis这边性能是不会提升的, 能节省是与客户端来回的网络IO. 要知道在一个分布式系统上实现一个原始语意的 mset, 这个相当于实现一个分布式事务, 性能肯定好不了. :)

###Codis 是多线程的吗?

这是个很有意思的话题, 由于 Redis 本身是单线程的, 跑的再欢也只能跑满一个核, 这个我们没办法, 不过 Redis 的性能足够好, 当value不是太大的时候仍然能达到很高的吞吐, 在CPU瓶颈之前更应该考虑的是带宽的瓶颈 (IO). 但是 Codis proxy 是多线程的(严格来说是 goroutine), 启动的线程数是 CPU 的核数, 是可以充分利用起多核的性能的.

###Codis 支持 CAS 吗? 支持 Lua 脚本吗?

CAS 暂时不支持, 但是如果非得支持, 我们可以考虑. Lua 脚本考虑到分布式环境下的一致性问题, 暂不支持.

###Codis的性能如何?

8 core Xeon 2.10GHz, 多线程的 benchmark, 单 proxy 的 qps 是 12w. 而且 proxy 是可以动态水平扩展的, 理论上的性能瓶颈应该是百万级别的.
由于 Codis 能很好的利用多核, 虽然在单核上的速度没有 Twemproxy 快, 但是并发数一上来, 你会发现单 Twemproxy 仅仅能吃满一个 CPU, 而 Codis 能尽可能的跑满 CPU. 所以在高并发/多线程客户端的场景下, Codis 的性能是不一定比单 Twemproxy 差, 见 benchmark. 但是 Codis 提供的最大价值是弹性扩缩容的能力, 不是吗 :) ?

###我的数据在 Codis 上是安全的吗?

首先, 安全的定义有很多个级别, Codis 并不是一个多副本的系统 (用纯内存来做多副本还是很贵的), 如果 Codis 底下的 redis 机器没有配从, 也没开 bgsave, 如果挂了, 那么最坏情况下会丢失这部分的数据, 但是集群的数据不会全失效 (即使这样的, 也比以前单点故障, 一下全丢的好...-_-|||). 如果上一种情况下配了从, 这种情况, 主挂了, 到从切上来这段时间, 客户端的部分写入会失败. 主从之前没来得及同步的小部分数据会丢失.
第二种情况, 业务短时间内爆炸性增长, 内存短时间内不可预见的暴涨(就和你用数据库磁盘满了一样), Codis还没来得及扩容, 同时数据迁移的速度小于暴涨的速度, 此时会触发 Redis 的 LRU 策略, 会淘汰老的 Key. 这种情况也是无解...不过按照现在的运维经验, 我们会尽量预分配一些 buffer, 内存使用量大概 80% 的时候, 我们就会开始扩容.

除此之外, 正常的数据迁移, 扩容缩容, 数据都是安全的. 
不过我还是建议用户把它作为 Cache 使用.

###Codis 的代码在哪? 是开源的吗?

是的, Codis 现在是豌豆荚的开源项目, 在 Github 上, Licence 是 MIT, 地址是:　https://github.com/wandoulabs/codis


###你们如何保证数据迁移的过程中多个 Proxy 不会读到老的数据 (迁移的原子性) ? 

见, [Codis 数据迁移流程]()