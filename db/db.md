# [Go] 用 net 包模拟数据库延迟

笔者想演示使用 errgroup 执行数据库查询对响应时间的影响。但因为数据库查询用时太低。导致演示的效果并不直观。且示例代码中也不会出现复杂 sql。所以只能另想办法。

笔者也尝试过 `time.Sleep`。但因为示例使用的是 gorm。所以尽管结果更直观了，但 gorm  自带的 logger 打出来的查询用时还是很小。

而笔者更想让 logger 打印出这个 “真实” 的执行用时。毕竟~~既然要追求刺激，就贯彻到底了~~。

所以笔者选择在 tcp 链接上做手脚。增加一个“中间人”，使用代码模拟出这个延时。也就是“中间商赚差价”。

实现代码如下：

``` go
package main

import (
	"io"
	"log"
	"net"
	"time"
)

func main() {
	l, err := net.Listen("tcp", ":8080")
	if err != nil {
		log.Fatalln(err)
	}
	defer l.Close()

	for {
		conn, err := l.Accept()
		if err != nil {
			log.Fatalln(err)
		}
		log.Println(conn.LocalAddr(), conn.RemoteAddr())
		go handle(conn)
	}
}

func handle(conn net.Conn) {
	defer conn.Close()

	db, err := net.Dial("tcp", ":3306")
	if err != nil {
		log.Fatalln(err)
	}
	defer db.Close()

	const d = 20 * time.Millisecond

	go io.Copy(conn, DelayReader(d, db))
	io.Copy(db, DelayReader(d, conn))
}

func DelayReader(d time.Duration, r io.Reader) io.Reader {
	return &delay{
		d: d,
		r: r,
	}
}

type delay struct {
	d time.Duration
	r io.Reader
}

func (r *delay) Read(p []byte) (n int, err error) {
	time.Sleep(r.d)
	return r.r.Read(p)
}
```

上面大都是些模板代码。其中值得关注的有以下两部分。

1. 通过两个 `io.Copy`  将客户端链接与数据库链接绑定在一起。

```go
go io.Copy(conn, DelayReader(d, db))
io.Copy(db, DelayReader(d, conn))
```

2. 自定义 io.Reader。用 `time.Sleep` 模拟延迟效果。

``` go
func (r *delay) Read(p []byte) (n int, err error) {
	time.Sleep(r.d)
	return r.r.Read(p)
}
```

这样只需更改一下端口，就能通过代码模拟出 sql 执行的延迟。笔者的演示代码数据也能更好看。

演示用的代码如下。这里将 gorm 换成了标准库的 db。

``` go
package main

import (
	"context"
	"database/sql"
	"fmt"
	"net/http"
	"time"

	_ "github.com/go-sql-driver/mysql"
	"golang.org/x/sync/errgroup"
)

const dsn = "root:root@tcp(localhost:8080)/test"

var db *sql.DB

func init() {
	db, _ = sql.Open("mysql", dsn)
	db.SetMaxIdleConns(5)
	db.SetMaxOpenConns(5)
}

func main() {
	var m http.ServeMux

	m.HandleFunc("/sync", func(rw http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		for i := 0; i < 5; i++ {
			query(ctx, db, i)
		}
	})

	m.HandleFunc("/async", func(rw http.ResponseWriter, r *http.Request) {
		eg, ctx := errgroup.WithContext(r.Context())
		for i := 0; i < 5; i++ {
			i := i
			eg.Go(func() error {
				return query(ctx, db, i)
			})
		}
		eg.Wait()
	})

	fn := http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
		start := time.Now()
		m.ServeHTTP(rw, r)
		fmt.Fprintf(rw, "%s %v\n", r.URL.Path, time.Since(start))
	})

	http.ListenAndServe(":9090", fn)
}

func query(ctx context.Context, tx *sql.DB, i int) error {
	var n int
	return tx.QueryRowContext(ctx, "SELECT ?", i).Scan(&n)
}
```

执行结果如下：
``` sh
$ curl http://localhost:9090/sync
/sync 255.369083ms

$ curl http://localhost:9090/async
/async 84.641167ms
```