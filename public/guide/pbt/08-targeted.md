# Targeted search and existence

Random inputs reach the common states of a system quickly and the rare ones slowly or never. Targeted property-based testing adds a utility function: the property reports how close a case came to something interesting, and pbt searches for cases with a higher utility instead of drawing fresh ones. PropEr calls it `?MAXIMIZE`.

## Maximize

```go
// A random walk rarely ends far to the right of the origin. Maximize turns
// the run into a search for walks that do, and the property fails once one
// gets past 50; without the search, 100 random cases would almost never
// fail.
var walkProperty = func(t *pbt.T) {
	steps := pbt.SliceOf(pbt.SampledFrom([]int{-1, 1})).Draw(t, "steps")
	pos := 0
	for _, s := range steps {
		pos += s
	}
	t.Maximize("position", float64(pos))
	if pos > 50 {
		t.Fatalf("the walk reached %d", pos)
	}
}
```

A random walk of `n` steps ends about `sqrt(n)` away from the origin, so 100 random walks almost never reach 50. With `t.Maximize("position", pos)`, the run turns into a search once the first case reports a utility. pbt mutates the choices of the best case so far, keeps a mutation when its utility is higher, sometimes keeps a worse one early in the search to escape a local maximum, and occasionally restarts from a fresh case. The search runs for the number of steps set by `SearchSteps` or `-pbt.searchsteps`, 1000 by default, instead of the number of checks.

```text
[pbt] property failed after 246 passed checks and 2039 shrink steps (size 33)
[pbt] utility position = 51
[pbt] steps (SliceOf(SampledFrom(2 values))) = []int{1, 1, 1, 1, …}
[pbt] error: the walk reached 51
```

The search found a walk past 50 after 246 cases, and the shrinker then reduced it to 51 steps to the right, the smallest input that fails. When the property passes, the summary reports the best utility seen. `t.Minimize` searches in the other direction. A second `Maximize` with the same name replaces the value of the case.

Use the search when a property has a notion of "closer": the depth of a tree, the length of a queue, the number of retries, the distance to a boundary. A utility that is flat over most inputs gives the search nothing to climb.

## Several objectives

A case can report several utilities under different names. The search then keeps a current and a best case for each objective, and the objectives take turns. Each step mutates the current case of one objective, in the order in which the objectives first appeared. It judges the mutation by that objective's utility alone.

```go
// simulate runs jobs of the given durations on a pool of workers and
// returns the longest queue and the longest wait.
func simulate(jobs []int, workers int) (longestQueue, longestWait int) {
	free := make([]int, workers) // the time at which each worker is free
	for i, d := range jobs {
		w := 0
		for k := range free {
			if free[k] < free[w] {
				w = k
			}
		}
		start := max(free[w], i) // job i arrives at time i
		longestWait = max(longestWait, start-i)
		waiting := 0
		for _, f := range free {
			if f > i {
				waiting++
			}
		}
		longestQueue = max(longestQueue, waiting)
		free[w] = start + d
	}
	return longestQueue, longestWait
}

var poolProperty = func(t *pbt.T) {
	jobs := pbt.SliceOf(pbt.IntRange(1, 10)).Draw(t, "jobs")
	workers := pbt.IntRange(1, 4).Draw(t, "workers")
	queue, wait := simulate(jobs, workers)
	t.Maximize("queue", float64(queue))
	t.Maximize("wait", float64(wait))
	if wait > 100 {
		t.Fatalf("a job waited %d", wait)
	}
}
```

```text
[pbt] property failed after 41 passed checks and 3005 shrink steps (size 26)
[pbt] utility queue = 1
[pbt] utility wait = 101
[pbt] jobs (SliceOf(IntRange(1, 10))) = []int{3, 10, 10, 10, 10, 10, 10, 10, 10, 10, 10, 10, 1}
[pbt] workers (IntRange(1, 4)) = 1
[pbt] error: a job waited 101
[pbt] reproduce: go test -run 'TestPool' -pbt.seed=0x1
```

The report lists every utility of the failing case. A passing run lists the best value of each objective in its summary. The objectives share the search steps, so give a run with several of them more steps with `SearchSteps`. A property with a single objective searches exactly as before.

## Existence

`CheckExists` inverts the question: instead of a property that must hold for every case, it looks for one case where the property holds. A case that fails is a rejected candidate, and the first case that passes is the witness, which pbt shrinks and logs.

```go
// CheckExists succeeds once a case passes: here, a Pythagorean triple. The
// witness is shrunk and logged. The seed keeps the run reproducible: a few
// seeds need more than 5000 cases to meet a triple.
func TestATripleExists(t *testing.T) {
	pbt.CheckExists(t, func(t *pbt.T) {
		a := pbt.IntRange(1, 50).Draw(t, "a")
		b := pbt.IntRange(1, 50).Draw(t, "b")
		c := pbt.IntRange(1, 70).Draw(t, "c")
		if a*a+b*b != c*c {
			t.Fail()
		}
	}, pbt.Checks(5000), pbt.Seed(1))
}
```

```text
[pbt] witness found after 98 rejected cases and 29 shrink steps (seed 0x1) (size 42)
[pbt] a (IntRange(1, 50)) = 3
[pbt] b (IntRange(1, 50)) = 4
[pbt] c (IntRange(1, 70)) = 5
```

The test fails when no witness turns up within the number of checks. `Maximize` works inside `CheckExists` too: a utility that measures the distance to a passing case guides the search toward one. Nothing is saved under `testdata/pbt` for an existential property, since a witness is not a regression.
