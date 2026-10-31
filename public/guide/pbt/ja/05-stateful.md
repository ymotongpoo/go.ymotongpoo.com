# ステートフルテスト

操作を何回か重ねて初めて現れるバグは多くあります。
追い出しの後に古い値を返すキャッシュ、クローズの後に数え間違えるキューなどです。
ステートフルテストは、操作をコマンドとして記述し、システムが保持しているはずの内容をモデルとして持ち、モデルに対してコマンド列を生成し、実際のシステムに対して実行して、すべての結果を検査します。
コマンド列が失敗すると、pbtはまだ失敗する最短の列へ縮小します。

## テスト対象のシステム

例はチケット番号を発行し、クローズするキューです。

```go
// Queue is the system under test: it issues ticket numbers and closes them.
type Queue struct {
	next int
	open map[int]bool
}

func NewQueue() *Queue { return &Queue{open: map[int]bool{}} }

func (q *Queue) Issue() int {
	q.next++
	q.open[q.next] = true
	return q.next
}

var errNotOpen = errors.New("ticket is not open")

func (q *Queue) Close(id int) error {
	if !q.open[id] {
		return errNotOpen
	}
	delete(q.open, id)
	return nil
}

func (q *Queue) Open() int { return len(q.open) }
```

## モデルとコマンド

モデルは、テストがシステムについて信じている内容です。
ここでは、開いているチケットの一覧です。
コマンド列を生成している間はまだ何も実行されていないので、チケット番号は存在しません。
そこでモデルはチケット番号を `pbt.Var` として保持します。
結果に付けるシンボリックな名前で、PropErの `{var, N}` と同じものです。

モデルとシステムはプロパティの中の変数で、コマンドはそれを参照するクロージャです。
値を受け渡す引数はなく、型パラメータも現れません。
プロパティがキューのコンストラクタを受け取るのは、同じコマンドを、ここでは正しいキューに、後の節ではバグのあるキューに対して動かすためです。

```go
// ticketQueue is what the commands need from the system under test.
type ticketQueue interface {
	Issue() int
	Close(id int) error
	Open() int
}

// queueProperty is the stateful property over a queue. The model is what
// the test believes about the queue: the tickets that are open, as Vars,
// because their numbers exist only once Issue has run. The model and the
// queue are variables of the property, and the commands close over them.
func queueProperty(newQueue func() ticketQueue) pbt.Property {
	return func(t *pbt.T) {
		q := newQueue()
		var open []pbt.Var

		issue := pbt.Just(pbt.Command{
			Name: "issue",
			Run:  func(pbt.Env) any { return q.Issue() },
			Next: func(v pbt.Var) { open = append(open, v) },
		})
		closeTicket := func() pbt.Gen[pbt.Command] {
			return pbt.Map(pbt.SampledFrom(open), func(v pbt.Var) pbt.Command {
				return pbt.Command{
					Name: "close", Args: []any{v},
					Pre:  func() bool { return slices.Contains(open, v) },
					Run:  func(env pbt.Env) any { return q.Close(pbt.Resolve[int](env, v)) },
					Next: func(pbt.Var) { open = slices.DeleteFunc(open, func(o pbt.Var) bool { return o == v }) },
					Post: func(_ pbt.Env, got any) error {
						if got != nil {
							return fmt.Errorf("close failed: %v", got)
						}
						return nil
					},
				}
			})
		}
		count := pbt.Just(pbt.Command{
			Name: "open",
			Run:  func(pbt.Env) any { return q.Open() },
			Post: func(_ pbt.Env, got any) error {
				if got != len(open) {
					return fmt.Errorf("%d open tickets, want %d", got, len(open))
				}
				return nil
			},
		})
		// commands chooses the next command from the model: close is only
		// offered while a ticket is open.
		commands := func() pbt.Gen[pbt.Command] {
			if len(open) == 0 {
				return pbt.OneOf(issue, count)
			}
			return pbt.Frequency(pbt.W(3, issue), pbt.W(2, closeTicket()), pbt.W(1, count))
		}
		init := func() { open = nil }

		prog := pbt.Commands(init, commands, pbt.ModelState(func() any { return open })).Draw(t, "cmds")
		prog.RunSequential(t)
	}
}

func TestQueue(t *testing.T) {
	pbt.Check(t, queueProperty(func() ticketQueue { return NewQueue() }))
}
```

コマンドは `pbt.Command` 型の値で、関数を束ねた構造体です。
各フィールドの役割は次のとおりです。

| フィールド | 役割 | 動く時点 |
| --- | --- | --- |
| `Name`、`Args` | 報告に表示される内容。呼び出しの引数を `Args` に入れ、結果を使う箇所にはその `Var` を入れると、報告がプログラムのように読める | 報告 |
| `Pre` | いまのモデル状態でコマンドが許されるかを示す。別のコマンドが取り除かれた結果として成り立たなくなったコマンドは、縮小の候補にならない | 生成時と、縮小した列の再実行時 |
| `Run` | システムに対して呼び出しを行う。`Env` はすべての `Var` をそのコマンドが返した値に対応づけ、`pbt.Resolve[T]` が型を付けて1つを読み出す | 実行時 |
| `Next` | コマンドに合わせてモデルを更新する。結果は `Var` として受け取るので、モデルに実際の値が入り込まず、縮小器が列を作り直せる | 生成時と実行時 |
| `Post` | コマンドの前のモデル状態に照らして実際の結果を検査する | 実行時 |

`pbt.Commands(init, gen)` はプログラムの生成器です。
`init` はモデルを初期状態に戻す関数で、pbtは生成の前と実行の前に毎回これを呼びます。
そのためモデルの変数をコピーする必要はありません。
`Next` はその場で書き換え、再生は `init` からやり直します。
関数 `gen` は、いまのモデル状態に対する次のコマンドの生成器を返します。
何が可能で、どれくらいの頻度かを決める場所です。
`close` はチケットが開いている間だけ提示され、`Frequency` が混合比を決めます。
`pbt.W` は重みと生成器を組にします。
オプション `pbt.Length(min, max)` は列の長さを制限し、`pbt.NoCommands()` を返す生成器は列を終え、`pbt.ModelState(f)` はモデルを表示する関数を報告に渡します。
`RunSequential` はプログラムを実行し、各事後条件を検査し、統計のためにすべてのコマンド名を記録します。
通過した実行では、各コマンドが何回動いたかが表示されます。

## 失敗を読む

次のキューはクローズしたチケットを取り除き忘れるので、`Open` がそれを数え続けます。

```go
// leakyQueue forgets to remove a closed ticket, so Open counts it still.
type leakyQueue struct{ Queue }

func (q *leakyQueue) Close(id int) error {
	if !q.open[id] {
		return errNotOpen
	}
	q.open[id] = false
	return nil
}
```

報告には、プログラム、実行の履歴、失敗したステップの前のモデル状態が載ります。

```text
[pbt] property failed after 5 passed checks and 65 shrink steps (size 6)
[pbt] cmds (Commands) = Program{
[pbt]     var1 = issue()
[pbt]     var2 = close(var1)
[pbt]     var3 = open()
[pbt]   }
[pbt] log: history:
[pbt] log:   1. var1 = issue() -> 1
[pbt] log:   2. var2 = close(var1) -> <nil>
[pbt] log:   3. var3 = open() -> 1
[pbt] log: model state before step 3: []
[pbt] error: step 3: postcondition of open() failed: 1 open tickets, want 0
[pbt] reproduce: go test -run 'TestQueue' -pbt.seed=0x1
```

最初に失敗した列には、発行とクローズが何回も含まれていました。
縮小器はコマンドを1つずつ取り除き、そのたびに事前条件を検査し直して、3つが残るまで続けました。
漏れを示す最短のプログラムです。
失敗は検査の種類とコマンド、つまり「postcondition of open」で区別されるので、縮小が別の失敗にすり替わることはありません。

## 良いモデルの書き方

モデルを変更するのは `init` と `Next` だけにしてください。
`Pre`、`Post`、`gen` はモデルを読むだけです。
pbtは `init` を呼んでから各コマンドの `Next` を順に呼ぶことで列を再生するので、それ以外の場所で加えた変更は再生時に失われます。
`init` が戻し忘れた値は、前のケースから次のケースへ漏れてしまいます。

モデルはシステムより単純に保ってください。
検査できる言葉でシステムがすべきことを述べるのが目的で、システムを作り直すことではありません。

コマンドは `Pre` で拒否するより、状態から選んでください。
頻繁に失敗する事前条件は生成したコマンドを無駄にしますが、状態に合ったコマンドを提示する生成器は何も無駄にしません。
`Pre` は縮小に必要な検査として残します。

統計も見てください。
`RunSequential` はすべてのコマンド名を記録するので、通過した実行を見れば、コマンドの混合比が意図どおりかが分かります。
