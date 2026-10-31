# 並行テスト

逐次的に使えば正しいシステムも、並行に使うと正しく動かないことがあります。
pbtは逐次テストと同じコマンドとモデルでそれを検査します。
逐次に実行する前段と、それに続く複数の枝を生成し、枝をゴルーチンで実行し、戻ってきた結果を説明できる枝の交互配置があるかを調べます。

```go
// Counter is safe for concurrent use. Every command returns a value the
// postcondition can check, which is what lets the interleaving check
// decide whether the concurrent run was explained by some sequential order.
type Counter struct {
	mu sync.Mutex
	n  int
}

func (c *Counter) Add(d int) int {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.n += d
	return c.n
}

func TestCounterParallel(t *testing.T) {
	pbt.Check(t, func(t *pbt.T) {
		c := &Counter{}
		n := 0 // the model: what the counter should hold
		add := pbt.Map(pbt.IntRange(1, 5), func(d int) pbt.Command {
			return pbt.Command{
				Name: "add", Args: []any{d},
				Run:  func(pbt.Env) any { return c.Add(d) },
				Next: func(pbt.Var) { n += d },
				Post: func(_ pbt.Env, got any) error {
					if got != n+d {
						return fmt.Errorf("add(%d) returned %v, want %d", d, got, n+d)
					}
					return nil
				},
			}
		})
		commands := func() pbt.Gen[pbt.Command] { return add }
		prog := pbt.ParallelCommands(func() { n = 0 }, commands).Draw(t, "cmds")
		prog.RunParallel(t)
	})
}
```

`ParallelCommands` は `Commands` と同じ `init` と `gen` を受け取り、前段の後に、それぞれ6コマンド以下の枝を2本持つプログラムを生成します。
オプション `pbt.Branches(n, maxLen)` は両方の数を指定します。
枝は3本まで、コマンドは全体で12個までです。
検査は交互配置を数え上げるので、数はすぐに膨らみます。
`pbt.Length` は前段の長さを制限します。
`RunParallel` は前段を実行し、次に枝を並行に実行し、最後に、記録した呼び出しと結果の順序のうち、すべての事後条件を順に満たすものを探します。
どの順序も満たさないとケースは失敗します。

```text
[pbt] error: no interleaving of the 2 branches satisfies the postconditions
```

この行の前に、プログラムと各枝の履歴が載ります。
枝の中でコマンドがパニックした場合もケースは失敗します。

## 検査の意味

検査は各枝の中の呼び出しの順序を保ち、枝どうしの間ではすべての順序を試します。
そのうちの1つがモデルと整合すれば、実行を受け入れます。
PropErの `parallel_commands` と同じ方式で、線形化可能性の検査より弱いものです。
線形化可能性の検査なら、異なる枝の呼び出しが完了した実時間の順序も尊重します。
この検査を通過しても、時刻を見る検査器だけが見つける形で線形化可能性に反することはあります。
それでも、よくある失敗は捕まえられます。
失われた更新、重複した識別子、途中の状態の読み出しです。

ここからコマンドの書き方について3つのことが決まります。

- すべてのコマンドは、事後条件が検査できる値を返さなければなりません。何も返さないコマンドは、検査に説明すべきものを与えません。
- 枝は別の枝の結果に依存できません。プログラムの生成時にはどちらも実行されていないからです。各枝は自分の `Var` と前段の `Var` だけを解決します。
- 枝の `Run` は並行に動くので、モデルに触れてはいけません。`Pre`、`Post`、`Next` はその後にテストのゴルーチンで動き、pbtは候補の順序ごとにモデルを再生します。

## レースディテクターと組み合わせる

交互配置の検査は、モデルが拒否する振る舞いを見つけます。
レースディテクターは、結果を変えたかどうかに関係なく、同期のないメモリアクセスを見つけます。
並行テストは `go test -race` で実行してください。
2つの道具は異なるバグを捕まえ、並行プロパティはレースディテクターが必要とする並行の負荷を与えます。

上の `Counter` はミューテックスを持っています。
これを外すと、実行によって、どの交互配置でも説明できない重複した結果が報告されるか、データレースが報告されます。
