## 结论

context 的意义就是在函数之间的调用过程中维护一个统一的 “上下文”。在 “上下文” 中使用保存 “状态” 的方式共享信息。

所谓的超时取消等 “功能” 也属于 “状态” 的一种。

且 context.Context ”碰巧“ 拥有了并发安全性。所以才会被广泛地应用到各种函数和方法中。

可以从几个方面来说明这个问题。

### 状态共享

用一个例子来说明状态共享的意义：

fn1 创建了 int 类型的 "状态" `i` 并将其传递了下去。

``` go
func fn1() {
	var i int

	fn2(i)
}
```

因为 fn4 需要 "状态" `i`。
所以尽管 fn2 fn3 本身并不需要这个 “状态”，但还是需要在参数里声明并传递这个 “状态”

``` go
func fn2(i int) {
	fn3(i)
}

func fn3(i int) {
	fn4(i)
}

func fn4(i int) {
	print(i)
}
```

最后，fn4 “消费” 了 “状态” `i`
至此，fn1 -> fn2 -> fn3 -> fn4 就像链条一样被 i 这个 “状态” 链接了起来。

到现在为止好像没有什么问题。各部分代码正常运行，只是稍微啰嗦了点。

但如果有一天需求发生变化，fn4 不需要 `i` 这个 “状态” 了呢？
那么从 fn4 到 fn2 间的所有代码都需要从参数声明中删除 `i` 这个状态。
这种蝴蝶效应似的代码改动无疑是巨大的。

为了解决这种 “明明只是改了最末尾的函数的参数列表，到最后却改动了整个工程的所有函数” 问题。
我们可以把 “获取状态” 这一行为单独抽象出来，**每个函数都从统一的 “参数中心” 中获取参数。**
这样就不用在需求改变的时候改动函数的声明了。



可以这样做：

1. 创建参数中心对象 ctx。
2. 修改函数，声明 ctx 作为参数。
3. 将“状态” `i` 共享到 ctx 中。
4. 从参数中心中获取状态。

``` go
type Context map[string]interface{}

func fn1() {
    var ctx = make(Context)
    ctx["i"] = 1

    fn2(ctx)
}

func fn2(ctx Context) {
    fn3(ctx)
}

func fn3(ctx Context) {
    fn4(ctx)
}

func fn4(ctx Context) {
    var i = ctx["i"].(int)
    print(i)
}
```

这样，无论函数需要多少参数，需求怎样变化，各函数都不需要修改自己的参数声明了。
而且像 fn2 fn3 这样的 “参数中转站” 也不会因为某个 “孙子辈” 的函数需求变化而导致自己修改了。
到这里为止，Context 对象的主要作用就解释清楚了。

### 流程控制

关于超时取消等上下文流程控制的问题，可以再举一个例子：

我们知道 channel 可以用来在协程之间传递信息。
``` go
var ch = make(chan int)

go func() {
    ch <- 1
}()

var i int = <-ch
print(i)
```

也知道 “channel 关闭” 也会作为一种信息被传递。
``` go
var ch = make(chan int)

go func() {
	ch <- 1
	close(ch)
}()

a, ok := <-ch // 1 true
b, ok := <-ch // 0 false
```

那么是不是可以创建一个 ”不处理数据，只处理关闭信号“ 的 channel 呢？
把 “channel 被关闭” 当作一种信息，在各个协程间传递。

``` go
var ch = make(chan struct{})

go func() {
    time.Sleep(time.Second) // 休眠 1s
    close(ch)
}

<-ch // 会阻塞 1s
```

所以，像 `ch := make(chan struct{})` 这样的 channel 可以用来在协程间传递一种代表 “结束” 的信号。
通过 `close(ch)` 的方式，**协程 A 可以在不接触协程 B 的情况下控制协程 B。**

### 状态封装

上面说到，可以使用 channel 类型的 ch 实现协程间的远程控制。

要是把“channel 类型的 ch”作为一种“状态”，放入 fn1 -> fn2 -> fn3 -> fn4 中。
那 fn1岂不是可以在不接触 fn4 的情况下就能控制 fn4 了？

可以这样做：

1. 先在 fn1 处创建一个 channel 类型的 “状态” ch。
2. 将状态 ch 分享到 “参数中心”。
3. 在 fn4 中监听 ch。

这样，fn1 就能通过 ch 远程控制 fn4 了。

但这样做会有一个小问题：
在 fn1 -> fn2 -> fn3 -> fn4 的调用链条中。fn2 和 fn3 也可以从参数中心中获取到状态 ch。
**在不经过 fn1 同意的情况下，fn2 和 fn3 可以私自结束掉 channel。**

明明是 fn1 创建的状态，其他函数却拥有完全访问权。颇有种 “你养的儿子管隔壁老王叫爹” 的感觉。~~还挺爽的~~

所以，**fn2 和 fn3 不能拥有完全控制状态 ch 的权限。**

解决思路也很简单。

在参数中心中共享 func 类型的状态 fn。封装 ch，只分享“监听结束信息”的”功能“就行了。

``` go
type Context map[string]interface{}

type Done func() (<-chan struct{})

func fn1() {
    var ch = make(chan struct{})

    // 函数
    var fn = func() (<-chan struct{}) {
        return ch
    }

    var ctx = make(Context)
    ctx["fn"] = fn

    go fn2(ctx)

    close(ch)
}

func fn2(ctx Context) {
    // 即使在这里截获 fn 也不能提前结束 ch
    // 因为 ch 被 fn 变向了
    fn3(ctx)
}

func fn3(ctx Context) {
    fn4(ctx)
}

func fn4(ctx Context) {
    fn := ctx["fn"].(Done)
    <-fn()
    print("done")
}

```

## 总结

总结一下：

1. 创建“参数中心”可以在函数调用链中共享状态、解除耦合。
2. 在参数中心共享 ch 可以实现跨协程控制。
3. 在参数中心共享 “功能” 可以实现状态的封装。

最后，如果把 fn1 -> fn2 -> fn3 -> fn4 的调用过程比喻成一条河流的话。那么 fn1 就可以称为 fn4 的 “上游”。

那么在 “上游” fn1 到下游 “fn4” 间传递的 “参数中心” ctx，就是对整个 “上下文” 进行控制的角色了。