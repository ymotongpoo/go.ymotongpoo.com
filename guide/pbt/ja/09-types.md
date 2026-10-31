# 型から導く生成器

プログラムのすべての構造体に生成器を書くのは手間で、多くの場合、値がどんな姿かは型がすでに語っています。
pbtは2つの方法で型から生成器を導きます。
実行時にリフレクションで組む `Make[T]` と、開発時にGoのソースとして書き出す `cmd/pbtgen` です。

## Make

```go
type Order struct {
	ID       int      `pbt:"min=1"`
	Items    []string `pbt:"min=1,max=5"`
	Note     string   `pbt:"-"` // stays zero
	Placed   time.Time
	internal int // unexported fields stay zero
}

var orderGen = pbt.Make[Order](
	pbt.WithType(pbt.Map(pbt.IntRange(0, 364), func(d int) time.Time {
		return time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC).AddDate(0, 0, d)
	})),
	pbt.WithField[Order]("Items", pbt.SliceOfN(pbt.StringN(1, 8), 1, 5)),
)

func TestOrders(t *testing.T) {
	pbt.Check(t, func(t *pbt.T) {
		o := orderGen.Draw(t, "order")
		if o.ID < 1 || len(o.Items) == 0 || o.Note != "" || o.Placed.Year() != 2026 {
			t.Fatalf("unexpected order %+v", o)
		}
	})
}
```

`Make[T]` は `T` のフィールドから再帰的に生成器を組みます。
`pbt` 構造体タグでフィールドを調整できます。
`min` と `max` は数値か、文字列、スライス、マップの長さを制限し、`-` はフィールドをゼロ値のままにします。
生成器を差し替えるオプションが2つあります。
`WithType(g)` はある型のすべての値に効き、上の例で `time.Time` に生成器を与えているのがこれです。
`WithField[S](name, g)` は構造体の1つのフィールドに効きます。

助けが要る型もあります。
リフレクションは非公開フィールドに値を設定できないので、非公開フィールドはゼロ値のままです。
状態がすべて非公開の型には `WithType` が要ります。
インターフェース、チャネル、関数には導出できる生成器がないので、同じく `WithType` か `WithField` が要ります。
足りないと `Make` はフィールド名を示してパニックします。

4章の `NoPanic` は、検査する関数の引数に `Make` を使い、同じオプションを受け付けます。

## 再帰的な型

自分自身を含む型は、何かが再帰を制限しない限り有限の生成器を持ちません。
`Recursive` には葉の生成器と、生成器を1段拡張する関数を渡します。
サイズパラメーターが値の段数を制限するので、1つの値のノード数はおおよそサイズになります。

```go
// Expr is a recursive type. Recursive bounds the depth with the size
// parameter, so a value has about size nodes instead of growing without end.
type Expr struct {
	Num  int
	Op   string
	Args []Expr
}

var exprGen = pbt.Recursive(
	pbt.Map(pbt.IntRange(0, 9), func(n int) Expr { return Expr{Num: n} }),
	func(self pbt.Gen[Expr]) pbt.Gen[Expr] {
		return pbt.Custom("op", func(t *pbt.T) Expr {
			return Expr{Op: pbt.SampledFrom([]string{"+", "*"}).Draw(t, ""), Args: pbt.SliceOfN(self, 2, 2).Draw(t, "")}
		})
	},
)

func size(e Expr) int {
	n := 1
	for _, a := range e.Args {
		n += size(a)
	}
	return n
}

func TestExprSize(t *testing.T) {
	pbt.Check(t, func(t *pbt.T) {
		e := exprGen.Draw(t, "expr")
		if size(e) > 4*t.Size()+8 {
			t.Fatalf("%d nodes at size %d", size(e), t.Size())
		}
	})
}
```

`Bounded(leaf, node)` は下位の形です。
値1つにつき1つの再帰予算を持ち、その値を組み上げる間に取り出すすべての制限付き生成器がその予算を共有します。
相互に再帰する2つの型に必要な仕組みです。
`Make` は再帰的な構造体に同じ予算を適用するので、自己参照する構造体が際限なく育つことはありません。

## pbtgen

`cmd/pbtgen` は生成器をソースとして書き出します。
コンパイラが検査でき、生成されたファイルはパッケージ自身の中にあるので非公開フィールドにも値を設定できます。

```text
//go:generate go run go.ymotongpoo.com/pbt/cmd/pbtgen -type Order,Customer
```

`go generate` は、各型 `T` について `pbt.Gen[T]` を返す関数 `GenT` を `pbt_gen.go` に書き出します。
`Make` の構造体タグはそのまま効きます。
`gen=EXPR` タグは、フィールドの生成器として使うGoの式を指定します。
同じパッケージにある `pbt.Gen[string]` 型の変数などです。
再帰的な型は、相互再帰を含めて `Bounded` を通して生成されます。

出力を調整するフラグが3つあります。
`-output` はファイル名を指定します。
`-prefix` は関数名の `Gen` を置き換えます。
pbtgenはパッケージがすでに宣言している名前を書き出さないので、手書きの生成器が上書きされることはありません。
pbtgenは現在のプラットフォームで `go build` がコンパイルするファイルを読むので、ビルド制約で除外されたファイルは生成されるコードに影響しません。

型が頻繁に変わるときや、生成器が1つのテストにだけ要るときは `Make` を選んでください。
生成器がパッケージのテスト支援の一部であるとき、非公開フィールドが重要なとき、テスト時のパニックよりコンパイルエラーのほうが望ましいときは、pbtgenを選んでください。
