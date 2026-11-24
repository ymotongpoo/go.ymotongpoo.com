# Generators

A generator is a value of type `pbt.Gen[V]`. It knows how to produce a `V` from the choices that pbt records for the current case, and that record is what makes shrinking and replay work. You never build the record yourself: you combine generators and draw from them.

## The built-in generators

| Values | Generators |
| --- | --- |
| Integers | `Int`, `IntRange`, `IntMin`, `IntMax`, `IntRangeToward`; `Int32`, `Int64`, `Uint`, `Uint32`, `Uint64`, `Byte` and their `Range` forms; `Integer[I]` and `IntegerRange[I]` for any integer type |
| Floats | `Float64`, `Float64Range`, `Float32`, `Float32Range` for finite values; `AnyFloat64` adds NaN and the infinities |
| Text | `Bool`, `Rune`, `RuneIn`, `String`, `StringN`, `StringOf`, `StringOfN`, `StringMatching`, `Bytes`, `BytesN` |
| Time | `Time`, `TimeRange`, `Duration`, `DurationRange`, `FixedZone`, `Zone` |
| Network | `IPv4`, `IPv6`, `Addr`, `AddrIn`, `Prefix`, `AddrPort` for `net/netip`, and `HardwareAddr` |
| Collections | `SliceOf`, `SliceOfN`, `SliceOfDistinct`, `SliceOfNDistinct`, `MapOf`, `MapOfN`, `Permutation` |
| Choice | `Just`, `SampledFrom`, `OneOf`, `Frequency`, `Ptr` |
| Functions | `Func`, `Func2`, which generate pure functions for properties over callbacks |
| Anything | `Any`, a bounded mix of common types, and `Make[T]`, which derives a generator from a type (chapter 9) |

Integers lean toward the boundaries of their range and toward small values, because those are where bugs live. Lengths of strings and collections follow the size parameter of the case. A maximum length of `-1` means no bound. Strings in the formats that standards define, such as RFC 3339 timestamps, e-mail addresses and URIs, come from the package `go.ymotongpoo.com/pbt/format`, which chapter 10 describes.

## Times and addresses

The generators of times and addresses follow the same rule as the integers: they lean toward the values where code breaks. `Time` draws an instant between the zero `time.Time` and the end of year 9999. It moves nearly half of the instants to an edge of the calendar: the first or last nanosecond of a day, a month or a year, February 29, or a bound of the range. In a share of the cases, it also rounds the fraction of a second to whole microseconds, milliseconds or seconds, since stored timestamps mostly carry those. A date one month ahead is a classic victim.

```go
var billingProperty = func(t *pbt.T) {
	start := pbt.Time().Draw(t, "")
	next := nextBillingDate(start)
	if want := (start.Month() % 12) + 1; next.Month() != want {
		t.Fatalf("next billing date %s is not in %s", next.Format(time.DateOnly), want)
	}
}
```

```text
[pbt] property failed after 39 passed checks and 13 shrink steps (size 40)
[pbt] start (Time()) = time.Date(1970, time.January, 31, 23, 59, 59, 999999999, time.UTC)
[pbt] error: next billing date 1970-03-03 is not in February
[pbt] reproduce: go test -run 'TestBilling' -pbt.seed=0x1
```

`AddDate` normalizes January 31 plus one month to March 3. The run found the last nanosecond of a month. The shrinker then moved it to January 1970, the month of the Unix epoch, which is where `Time` shrinks. `TimeRange(lo, hi)` keeps values in a range and in the location of `lo`. In a location with daylight saving time, it also moves values next to a change of offset: to the moment of the change, the nanosecond before it, or the half hours around it. `Zone` loads IANA time zones, by default a list of zones with unusual rules, and `FixedZone` draws fixed offsets.

The address generators return the types of `net/netip`. `IPv4` and `IPv6` draw a share of their addresses from the special-purpose blocks of RFC 6890. Among them are loopback, private networks, link local, documentation and multicast, and, for IPv6, IPv4-mapped addresses. `AddrIn` keeps addresses inside the prefixes you give it.

```go
// The client of a request is an address inside the private networks of
// RFC 1918, and the rate limiter keys clients by their /24.
func TestClientKey(t *testing.T) {
	private := pbt.AddrIn(
		netip.MustParsePrefix("10.0.0.0/8"),
		netip.MustParsePrefix("172.16.0.0/12"),
		netip.MustParsePrefix("192.168.0.0/16"),
	)
	pbt.Check(t, func(t *pbt.T) {
		client := private.Draw(t, "client")
		key := netip.PrefixFrom(client, 24).Masked()
		if !key.Contains(client) || !key.Addr().IsPrivate() {
			t.Fatalf("key %s does not cover %s", key, client)
		}
	})
}
```

## Strings that match a pattern

`StringMatching` builds strings from a regular expression in the syntax of package `regexp`. Every value matches the whole pattern, and shrinking keeps it matching. Alternations shrink toward earlier branches, repetitions toward their minimum count, and character classes toward their first printable ASCII member. `RuneIn` draws runes from the range tables of package `unicode`, such as scripts and categories. Every range of the tables gets the same chance, so a small script is not drowned out by the CJK ideographs.

```go
// An order number has two capital letters, a dash and four digits.
var orderID = regexp.MustCompile(`^[A-Z]{2}-\d{4}$`)

func TestOrderIDs(t *testing.T) {
	pbt.Check(t, func(t *pbt.T) {
		id := pbt.StringMatching(`[A-Z]{2}-\d{4}`).Draw(t, "id")
		if !orderID.MatchString(id) {
			t.Fatalf("%q is not an order number", id)
		}
		name := pbt.StringOfN(pbt.RuneIn(unicode.Hiragana, unicode.Katakana, unicode.Han), 1, 8).Draw(t, "name")
		if label := fmt.Sprintf("%s %s", id, name); len([]rune(label)) > 16 {
			t.Fatalf("label %q is too long", label)
		}
	})
}
```

## Building your own

`pbt.Custom` turns a function of `*pbt.T` into a generator. Inside it you draw from other generators, in any order and under any control flow.

```go
type User struct {
	Name  string
	Age   int
	Email string
}

// userGen builds a User from smaller generators. Labeled draws inside a
// generator appear in the report, indented under the value they build.
var userGen = pbt.Custom("user", func(t *pbt.T) User {
	name := pbt.StringOfN(pbt.SampledFrom([]rune("abcdefghijklmnopqrstuvwxyz")), 1, 12).Draw(t, "name")
	return User{
		Name:  name,
		Age:   pbt.IntRange(0, 120).Draw(t, "age"),
		Email: name + "@example.com",
	}
})
```

The name given to `Custom` is how the generator appears in reports. The labels of the draws inside it matter too: a labeled draw inside a generator is listed in the report, indented under the value it helped to build. Unlabeled draws, with `""`, are not listed, which is right for a value that is not interesting on its own.

## Composing generators

Three functions cover most compositions.

```go
// Map transforms a value, Filter keeps the values that satisfy a predicate,
// and Bind picks the next generator from a value.
var (
	even    = pbt.Map(pbt.IntRange(0, 500), func(n int) int { return 2 * n })
	nonZero = pbt.Filter(pbt.Int(), func(n int) bool { return n != 0 })
	matrix  = pbt.Bind(pbt.IntRange(1, 4), func(n int) pbt.Gen[[][]int] {
		row := pbt.SliceOfN(pbt.Int(), n, n) // every row has the same length
		return pbt.SliceOfN(row, 1, 4)
	})
)
```

- `Map` applies a function to each value. The result shrinks as the input does.
- `Filter` discards the values that fail a predicate and draws again. A predicate that rejects most values makes the run slow, and pbt gives up on a case after many rejections. Prefer building the value you want: `IntRange(1, 100)` over `Filter(Int(), positive)`.
- `Bind` chooses the next generator from a value, which is how one part of a value can depend on another. PropEr calls it `?LET`.

`OneOf` picks one of several generators with equal probability and `Frequency` with weights; chapter 2 covers them. `Ptr` wraps a value in a pointer, with or without `nil`. `Lazy` defers the construction of a generator until it is first drawn from, which is what a recursive generator needs to refer to itself; chapter 9 shows the bounded form.

## Reports show how a value was built

A property that fails on a `User` reports the user and, under it, the labeled parts.

```go
var displayNameProperty = func(t *pbt.T) {
	u := userGen.Draw(t, "user")
	display := strings.ToUpper(u.Name[:1]) + u.Name[1:]
	if unicode.IsLower(rune(display[0])) {
		t.Fatalf("display name %q is not capitalized", display)
	}
	if len(display) > 8 {
		t.Fatalf("display name %q is too long for the badge", display)
	}
}
```

```text
[pbt] property failed after 3 passed checks and 189 shrink steps (size 4)
[pbt] user (user) = snippets.User{Name:"aaaaaaaaa", Age:0, Email:"aaaaaaaaa@example.com"}
[pbt]   name (StringOfN(SampledFrom(26 values), 1, 12)) = "aaaaaaaaa"
[pbt]   age (IntRange(0, 120)) = 0
[pbt] error: display name "Aaaaaaaaa" is too long for the badge
[pbt] reproduce: go test -run 'TestDisplayName' -pbt.seed=0x1
```

The name shrank to the shortest string that still breaks the length check, nine letters of `a`, and the age to 0 because it played no part in the failure. A report lists labeled draws up to three levels deep and cuts each value at 2000 characters; the `ReportDepth` and `ReportLimit` options change both.

A draw at the top level of the property appears in the report even without a label. When the draw is assigned to a single variable, as in `start := pbt.Time().Draw(t, "")` above, the report names the value after the variable. Otherwise it gives the file and the line of the draw, as in `Int() at prop_test.go:12 = 5`. The report reads the source file to find the variable. When the file is not available, for example in a binary built with `-trimpath`, the report gives the line. `Verbose()` or `-pbt.v` adds the line to every draw.

## Sampling outside a property

`Example` returns one value of a generator, for a quick look or for a conventional test that needs a plausible input. A seed makes the value stable for the same generator code and pbt version.

```go
func TestExample(t *testing.T) {
	u := userGen.Example(42) // a sample outside any property; the seed fixes it
	if u.Age < 0 || !strings.HasSuffix(u.Email, "@example.com") {
		t.Fatalf("unexpected user %+v", u)
	}
}
```

## Values that generators share

`Just(v)` returns the same `v` for every case. When `v` is a slice, a map or a pointer, every case sees the same object, so a property that modifies it changes what later cases and the report see. `SampledFrom(values)` copies the slice but hands out the elements as they are. Draw a fresh value from `Custom` when the property needs to modify it.
