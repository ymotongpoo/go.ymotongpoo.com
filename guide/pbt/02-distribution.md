# Size, distribution and statistics

pbt lets you decide what the data looks like, and gives you the tools to check that it looks the way you intended. Both halves matter: a property that never reaches the interesting states of a system proves nothing about them.

## The size parameter

Every test case runs at a size. `Check` starts at size 1 and raises it by one for each passing case, up to 42, so a run begins with small inputs and ends with large ones. The lengths of strings, slices and maps average the size; the magnitude of integers grows with it too. `t.Size()` returns the size of the current case, and the `Sizes(start, max)` option or the `-pbt.startsize` and `-pbt.maxsize` flags change the range.

`Sized` builds a generator from the size, which is how PropEr's `?SIZED` works, and `Resize` runs a generator at a fixed size. Together they bound a recursive structure.

```go
// A tree whose depth follows the size parameter: small early cases, larger
// later ones. Resize fixes the size of a part of a value.
type Tree struct {
	Value    int
	Children []Tree
}

func tree() pbt.Gen[Tree] {
	return pbt.Sized(func(size int) pbt.Gen[Tree] {
		if size <= 1 {
			return pbt.Map(pbt.Int(), func(v int) Tree { return Tree{Value: v} })
		}
		return pbt.Custom("tree", func(t *pbt.T) Tree {
			child := pbt.Resize(size/2, tree())
			return Tree{Value: pbt.Int().Draw(t, ""), Children: pbt.SliceOfN(child, 0, 3).Draw(t, "")}
		})
	})
}
```

Halving the size at every level gives a tree whose depth is at most the logarithm of the size, whatever the branching. `Resize(0, g)` makes collections empty, which is the smallest case of anything built from them.

## Weights

`Frequency` draws one of its alternatives with a probability proportional to its weight.

```go
type Op struct {
	Kind string
	Key  string
}

// ops reads four times as often as it writes; shrinking moves toward the
// first alternative, the read.
var ops = pbt.Frequency(
	pbt.W(4, pbt.Map(pbt.StringN(1, 3), func(k string) Op { return Op{"get", k} })),
	pbt.W(1, pbt.Map(pbt.StringN(1, 3), func(k string) Op { return Op{"put", k} })),
)
```

Weights only affect generation. Shrinking always moves toward the first alternative, whatever the weights are, so the order of the alternatives is the shrinking priority. An alternative with weight 0 is never generated and is reachable only by shrinking, which is what `ShrinkTo` in chapter 3 is built on. `OneOf` is `Frequency` with equal weights.

## Statistics

Four methods on `*pbt.T` describe the cases of a run. `Check` prints the result when the property passes.

```go
func TestOperationMix(t *testing.T) {
	pbt.Check(t, func(t *pbt.T) {
		batch := pbt.SliceOfN(ops, 1, 20).Draw(t, "batch")
		writes := 0
		for _, op := range batch {
			t.Aggregate(op.Kind) // share of each kind among all operations
			if op.Kind == "put" {
				writes++
			}
		}
		t.Classify(writes == 0, "read-only batch") // share of cases
		t.Measure("batch length", float64(len(batch)))
		t.Cover(writes > 0, 60, "has a write")
	}, pbt.CheckCoverage())
}
```

- `Classify(cond, label)` counts the cases for which the condition holds, and `Collect(v)` counts the cases by the text of a value. Both report a share of the cases.
- `Aggregate(values...)` counts samples, several per case, and reports their share of all samples. It is the right call for the names of the commands a stateful test ran.
- `Measure(name, value)` records a number and reports its minimum, average and maximum.

```text
[pbt] OK, passed 100 checks (seed 0x1)
[pbt]  22.0% read-only batch
[pbt]  81.1% get
[pbt]  18.9% put
[pbt] batch length: min 1, avg 12.43, max 20 (100 samples)
```

The shares confirm the four-to-one weighting, and the length line confirms that batches of the full length occur. `Verbose()` or `-pbt.v` prints the summary even when the property records nothing.

## Checking coverage

`Cover(cond, pct, label)` turns an observation into a requirement: the cases for which `cond` holds must make up at least `pct` percent of all cases. The example above requires a write in 60% of the batches.

The decision is statistical, since a run of 100 cases can miss a 60% share by chance. pbt computes the 99.7% confidence interval of the observed share, the Wilson score interval. The requirement is met when the whole interval lies above `pct`, unmet when it lies below, and undecided while it straddles `pct`. Without the `CheckCoverage` option, `Check` logs a shortfall and passes. With it, `Check` keeps running cases past the configured number, up to twenty times as many, until every requirement is decided. It then fails on a shortfall, and it also fails when a requirement is still undecided, so a pass means that the data supports the share.

```go
var rareCoverProperty = func(t *pbt.T) {
	n := pbt.IntRange(0, 999).Draw(t, "n")
	t.Cover(n%100 == 0, 30, "round hundred") // too rare for a 30% share
}
```

```text
[pbt] insufficient coverage:
[pbt]   round hundred: 17.6% of 125 cases, required 30.0%
```

The run stopped after 125 cases, once the interval excluded 30%. The observed share also says something about `IntRange`: 0 is a boundary value, so it occurs far more often than one case in a thousand. Statistics are how you find that out, and `Frequency` or `SampledFrom` are how you change it when a uniform spread is what the property needs.
