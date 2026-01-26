# Go Coding Style Conventions

## Core Principles

Priorities for readable code (in order):
1. **Clarity**: Purpose and rationale are obvious to the reader
2. **Simplicity**: Code is straightforward
3. **Concision**: Minimal without sacrificing clarity
4. **Maintainability**: Easy to modify and extend
5. **Consistency**: Follow established patterns
6. **Avoid duplication**: Extract shared logic into common helpers or utilities rather than duplicating code across packages.

## Happy Path Coding

Structure code so the successful path flows straight down. Handle errors immediately, then continue with main logic. Avoid nesting main logic inside conditional branches.

## Error Handling

- **Avoid panics**: Production code must return errors, allowing callers to decide how to handle them
- **Error wrapping**: Always wrap errors with context using `%w` verb; use `%v` to hide implementation details when callers shouldn't inspect underlying errors
- **Error messages**: Keep context brief; avoid redundant phrases like "failed to" that accumulate through the call stack
- **Error formatting**: Error strings should not be capitalized or end with punctuation (they often appear mid-sentence)
- **Multiple errors**: Use `errors.Join()` (Go 1.20+) when independent operations fail
- **Error naming**: Exported error variables use `Err` prefix; custom error types use `Error` suffix
- **Sentinel errors**: Define package-level sentinel errors with `errors.New()`; check with `errors.Is()` rather than equality
- **Single responsibility**: Handle errors once—either log or return, not both, to prevent duplicate logging

## Naming Conventions

- **MixedCaps always**: Go uses MixedCaps, never underscores
- **Initialisms**: Maintain consistent casing (e.g., `URL` not `Url`, `ID` not `Id`)
- **Variable scope**: Short names for limited scope; longer scope requires more descriptive names
- **Receiver names**: Use one or two-letter type abbreviations consistently; avoid `this` or `self`
- **Package names**: Single-word lowercase names; avoid generic terms like `util`, `common`, or `misc`
- **Avoid repetition**: Don't repeat package or receiver names in function names
- **Avoid shadowing**: Don't shadow predeclared identifiers (`new`, `make`, `len`, `copy`, `append`, `true`, `false`, `nil`, `error`, `string`, `int`, `bool`, etc.)

## Comments and Documentation

- All comments end with a period.
- Comments documenting declarations should be full sentences starting with the name being described.
- Package comments go before the package declaration with no blank line between them.
- Use `go doc` commands to reference standard library and package documentation.

## Line Length

Maximum line length is 120 characters. Break extended lines at logical points, particularly in function signatures and long error messages.

## Structs and Initialization

- **Named fields**: Always use field names when initializing; positional arguments break when fields change
- **Omit zero values**: Don't explicitly initialize fields to their zero values
- **Avoid embedding**: Don't embed types in public structs; this unintentionally exposes methods to the API
- **Zero-value initialization**: Prefer `var` for zero-value structs

## Slices and Maps

- **Nil preference**: Prefer nil slices over empty slices; they are functionally equivalent but nil is preferred
- **Copy at boundaries**: Copy slices and maps when storing or returning to prevent external mutation
- **Preallocate capacity**: Specify capacity when size is known to reduce allocations
- **Map capacity**: Specify capacity when map size is known
- **Standard library functions**: Use `slices` and `maps` packages for common operations

## Imports

- **Organization**: Group imports in three sections separated by blank lines:
  1. Standard library
  2. External packages
  3. Internal packages
- **Rename minimally**: Only rename imports to avoid collisions
- **Avoid dot imports**: Don't use `import . "pkg"` except in test files with circular dependencies
- **Side-effect imports**: Only import for side effects in main packages or tests using `import _`

## Concurrency

- **Synchronous default**: Functions should be synchronous; let callers add concurrency when needed
- **Channel sizing**: Use unbuffered channels (size 0) or size 1; any other size requires justification
- **Goroutine lifecycle**: Document how and when goroutines exit; blocked goroutines prevent garbage collection
- **Error handling**: Prefer `errgroup.Group` over manual `sync.WaitGroup` for concurrent operations
- **Mutexes**: Zero-value `sync.Mutex` is valid; avoid pointers to mutexes or embedding them in exported structs
- **Atomic operations**: Use typed atomics from `sync/atomic` (Go 1.19+)

## Generics (Go 1.18+)

- **Use when appropriate**: Implement generics when writing identical code for different types
- **Type constraints**: Use constraints like `cmp.Ordered` for type safety
- **Avoid over-generalization**: Use concrete types or interfaces instead of generics when they suffice

## Testing

- **Table-driven tests**: Avoid duplication using table-driven patterns; run subtests in parallel when safe
- **Clear failures**: Include inputs, actual results, and expected results in test error messages
- **Use t.Fatal for setup**: Call `t.Fatal()` if test setup fails
- **Interface placement**: Define interfaces in the consumer package, not the provider
- **Comparison tools**: Prefer `github.com/google/go-cmp/cmp` over `reflect.DeepEqual`

## Performance

- **Prefer strconv**: Use `strconv` over `fmt` for primitive conversions—it's faster
- **Avoid repeated conversions**: Convert strings to bytes once, then reuse
- **String concatenation**: Use `strings.Builder` instead of repeated string concatenation

## Resource Management

- **Defer for cleanup**: Use `defer` to clean up resources for safety and readability
- **Context as first parameter**: Context should be the first parameter, named `ctx`
- **Avoid storing context**: Don't store context in structs

## Common Patterns

- **Functional options**: Use optional parameter functions for configurable constructors with many options
- **Interface verification**: Use compile-time checks: `var _ InterfaceName = (*Implementation)(nil)`
- **Graceful shutdown**: Production servers need proper shutdown to drain connections
- **Enum zero values**: Start enums at 1; let 0 represent invalid or unset state
- **Time values**: Use `time.Time` for instants and `time.Duration` for periods—never bare integers
- **Type assertions**: Always use the two-value form to avoid panics
- **Dependency injection**: Use explicit initialization instead of mutable globals

## Gotchas

- **Defer evaluation**: Defer evaluates arguments immediately, not when the deferred function runs
- **Nil interfaces**: An interface containing a nil pointer is not nil; return `nil` explicitly
- **Map iteration**: Map iteration order is randomized; don't depend on order
- **Slice append**: Append may reuse the backing array; use full slice expressions `a[:n:n]` to limit capacity
- **Error checking**: Always check errors before using results to avoid panics
