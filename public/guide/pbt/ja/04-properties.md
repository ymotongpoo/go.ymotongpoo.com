# プロパティと実行

実行の形を決める要素を扱います。
ケースの数、サイズ、破棄したケースの扱い、ケースの前後に動くもの、プロパティの組み合わせ、失敗の再現です。

## オプションとフラグ

すべての設定に、`go test` の実行全体に有効なフラグと、1回の `Check` に有効なオプションがあります。
オプションがフラグより優先されます。

| フラグ | オプション | デフォルト | 意味 |
| --- | --- | --- | --- |
| `-pbt.checks` | `Checks(n)` | 100 | 必要な通過ケース数 |
| `-pbt.seed` | `Seed(s)` | ランダム | 実行のシード。すべての報告に表示 |
| `-pbt.startsize`、`-pbt.maxsize` | `Sizes(start, max)` | 1、42 | 最初のケースのサイズと、増加の上限 |
| `-pbt.shrinktime` | `ShrinkTime(d)` | 30s | 失敗1件の縮小に使う時間 |
| `-pbt.searchsteps` | `SearchSteps(n)` | 1000 | 標的型探索のケース数 |
| `-pbt.nosave` | `NoSave()` | false | 失敗したケースを保存しない |
| `-pbt.replay` | `ReplayFile(path)` | なし | 探索の前に再実行する保存済みケース |
| `-pbt.v` | `Verbose()` | false | 通過した実行の要約と縮小の各ステップをログに出す |
| | `TestdataDir(dir)` | `testdata/pbt` | 失敗したケースの保存先 |
| | `NoReplay()` | false | 保存済みケースを再実行しない |
| | `Setup(f)` | なし | 各ケースの前に `f` を実行 |
| | `CheckCoverage()` | false | 不足か未決の `Cover` 要求で失敗 |
| | `ReportLimit(chars)`、`ReportDepth(levels)` | 2000、3 | 報告する値の文字数と、入れ子の値の深さ |

```go
func TestSortIsIdempotent(t *testing.T) {
	pbt.Check(t, func(t *pbt.T) {
		xs := pbt.SliceOf(pbt.Int()).Draw(t, "xs")
		once := slices.Sorted(slices.Values(xs))
		twice := slices.Sorted(slices.Values(once))
		if !slices.Equal(once, twice) {
			t.Fatalf("sorting twice gave %v, then %v", once, twice)
		}
	}, pbt.Checks(500), pbt.Sizes(1, 60))
}

// MakeCheck turns a property into a test function, for table-driven or
// generated tests.
var TestReverseTwice = pbt.MakeCheck(func(t *pbt.T) {
	xs := pbt.SliceOf(pbt.Rune()).Draw(t, "xs")
	ys := slices.Clone(xs)
	slices.Reverse(ys)
	slices.Reverse(ys)
	if !slices.Equal(xs, ys) {
		t.Fatalf("reversing twice changed %v", xs)
	}
})
```

`MakeCheck` はプロパティを `func(*testing.T)` に包みます。
プロパティの表やサブテストに向いています。

## ケースを破棄する

`t.Assume(cond)` は条件が偽のとき現在のケースを破棄します。
PropErの `?IMPLIES` と同じです。
破棄したケースは通過にも失敗にも数えず、`Check` は別のケースを引きます。
`Skip`、`Skipf`、`SkipNow` も同じ働きです。
そのケースですでに失敗を記録していれば、後から破棄しても失敗は残ります。

```go
func TestDivision(t *testing.T) {
	pbt.Check(t, func(t *pbt.T) {
		a := pbt.Int().Draw(t, "a")
		b := pbt.Int().Draw(t, "b")
		t.Assume(b != 0) // discards the case; too many discards fail the run
		if (a/b)*b+a%b != a {
			t.Fatalf("division identity fails for %d / %d", a, b)
		}
	})
}
```

破棄には予算があります。
破棄がチェック数の10倍を超えると、最後に破棄した理由を示して実行は失敗します。
大半のケースを破棄するプロパティは、主張しているより少ないことしか検査していないからです。
`Assume(b > 0)` より `IntMin(1)` のように、有効な入力を作る生成器があるならそちらを選んでください。

## セットアップとクリーンアップ

`Setup` オプションは各ケースの前に関数を実行します。
その関数は後片付けの関数を返してもよく、返した関数はケースの後に実行されます。
`t.Cleanup` はプロパティの中から同じ目的の関数を登録します。
クリーンアップは、ケースが失敗したかどうかに関係なく、ケースの後に登録と逆の順序で実行されます。

あるクリーンアップでパニックや破棄が起きても、残りのクリーンアップは実行されます。
プロパティがケースを破棄していても、クリーンアップで失敗すればケースは失敗になります。
クリーンアップで破棄しても、プロパティの失敗やパニックは消えません。
プロパティとクリーンアップの両方でパニックが起きた場合、報告にはプロパティのパニックが残ります。

```go
func TestFileRoundTrip(t *testing.T) {
	dir := t.TempDir()
	withFile := pbt.Setup(func(t *pbt.T) func() {
		path := filepath.Join(dir, "case")
		t.Cleanup(func() { os.Remove(path) }) // Cleanup runs after the case, latest first
		return nil                            // or return a teardown function
	})
	pbt.Check(t, func(t *pbt.T) {
		data := pbt.Bytes().Draw(t, "data")
		path := filepath.Join(dir, "case")
		if err := os.WriteFile(path, data, 0o644); err != nil {
			t.Fatal(err)
		}
		got, err := os.ReadFile(path)
		if err != nil || !slices.Equal(got, data) {
			t.Fatalf("read back %v (%v), want %v", got, err, data)
		}
	}, withFile)
}
```

## プロパティを組み合わせる

`Conjunction` は名前付きの複数のプロパティを同じケースで実行し、失敗したものの名前を示します。
後続のプロパティがケースを破棄しても、先行するプロパティの失敗は消えません。
`Deadline` は所定の時間より長くかかったケースを失敗にします。

```go
func TestStringProperties(t *testing.T) {
	// Conjunction runs every sub-property on the same case and names the
	// ones that fail.
	pbt.Check(t, pbt.Conjunction(
		pbt.Named{Name: "upper is idempotent", Prop: func(t *pbt.T) {
			s := pbt.String().Draw(t, "s")
			if strings.ToUpper(strings.ToUpper(s)) != strings.ToUpper(s) {
				t.Fatalf("%q", s)
			}
		}},
		pbt.Named{Name: "fields never contain spaces", Prop: func(t *pbt.T) {
			s := pbt.String().Draw(t, "s")
			for _, f := range strings.Fields(s) {
				if strings.ContainsRune(f, ' ') {
					t.Fatalf("%q", f)
				}
			}
		}},
	))

	// Deadline fails a case that takes too long. Go cannot stop a running
	// function, so the check happens after the property returns.
	pbt.Check(t, pbt.Deadline(time.Second, func(t *pbt.T) {
		s := pbt.StringN(0, 100).Draw(t, "s")
		if strings.Repeat(s, 2) != s+s {
			t.Fatal("repeat")
		}
	}))
}
```

連言の失敗には、失敗した要素の名前が付きます。

```text
[pbt] error: non-empty: empty
[pbt] error: conjunction: [non-empty] failed
```

Goは実行中の関数を中断できないので、`Deadline` はプロパティが戻った後に経過時間を検査します。
戻ってこないプロパティは `go test -timeout` が捕まえます。
`Timeout` は `Deadline` の以前の名前で、引き続き使えます。

## 失敗を説明する

`t.OnFail(f)` は、報告されるケースについてだけ、縮小の後に実行される関数を登録します。
そこでログに出した内容は報告に載ります。
テスト対象のシステムの状態や計算の途中の値を表示する場所で、PropErの `?WHENFAIL` に当たります。

```go
func TestOnFail(t *testing.T) {
	pbt.Check(t, func(t *pbt.T) {
		xs := pbt.SliceOf(pbt.Int()).Draw(t, "xs")
		sum := 0
		t.OnFail(func() { t.Logf("partial sum when it failed: %d", sum) }) // runs only for the reported case
		for _, x := range xs {
			sum += x
		}
		if len(xs) == 0 && sum != 0 {
			t.Fatal("impossible")
		}
	})
}
```

## パニック

プロパティの中のパニックはケースを失敗にします。
報告にはパニックの値と、あなたのコードのフレームが載ります。
縮小器は同じ場所でパニックする最小のケースを探します。
パニックそのものを扱うプロパティのために、補助の関数が2つあります。

```go
func mustPositive(n int) int {
	if n <= 0 {
		panic("not positive")
	}
	return n
}

func TestPanics(t *testing.T) {
	// Recover turns a panic into a value for a property to assert on.
	pbt.Check(t, func(t *pbt.T) {
		n := pbt.IntMax(0).Draw(t, "n")
		if p := pbt.Recover(func() { mustPositive(n) }); p == nil {
			t.Fatalf("mustPositive(%d) did not panic", n)
		}
	})
	// NoPanic generates the arguments of a function from its signature and
	// fails when a call panics.
	pbt.Check(t, pbt.NoPanic(strings.Repeat, pbt.WithType(pbt.IntRange(0, 3))))
}
```

`Recover` は関数を実行し、パニックの値か `nil` を返すので、不正な入力でパニックすることをプロパティで主張できます。
`NoPanic` は関数を受け取り、その引数を `Make` で生成します。
`Make` は9章で説明します。
`WithType` と `WithField` のオプションで引数を絞れます。

## シード、保存したケース、再実行

すべての報告は、実行のシードを含む `reproduce:` の行で終わります。
そのシードを `-pbt.seed` に渡すと、プロパティのコードとpbtのバージョンが同じであれば、失敗したケースを含む同じケースの列が繰り返されます。
pbtを更新すると、同じシードから生成されるケースが変わることがあります。
v0.3.2では整数生成の修正により一部のシードで生成結果が変わりますが、既存の保存済み選択列は従来どおり再生されます。

失敗したケースは `testdata/pbt/<テスト名>/<ハッシュ>` にもテキストファイルとして保存されます。
サイズ、コメントとして取り出した値、そして選択の列が入っています。
同じテストの以後の `Check` は、新しいケースを生成する前に保存済みのケースを再実行し、まだ失敗するなら失敗にします。
回帰ケースがコードとともにバージョン管理に入るわけです。
ケースを捨てるにはファイルを削除します。
この挙動を調整するオプションが3つあります。
`NoSave` は実行に書き込みをさせません。
`NoReplay` は保存済みケースを飛ばします。
`ReplayFile` またはフラグ `-pbt.replay=FILE` は、どのテストでも保存済みファイルを1つ再実行します。
あるテストのケースを別のテストに渡すときや、CIのログにある失敗を手元で再現するときに使います。
