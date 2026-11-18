# Rust crate assembly fixture

Add the dependencies:

```toml
[dependencies]
# Integer formatting for the order IDs.
itoa = "=1.0.15"
```

Add 1 more dependency without a table header:

```toml
ryu = "=1.0.20"
itoa = "=1.0.15"
```

Write the formatting module:

```rust
// src/format.rs
pub fn order_id(n: u32) -> String {
    let mut buffer = itoa::Buffer::new();
    format!("TEST-{:0>4}", buffer.format(n))
}

pub fn amount(value: f64) -> String {
    let mut buffer = ryu::Buffer::new();
    buffer.format(value).to_string()
}
```

The module is not declared by main, so the assembler declares it:

```rust
// src/endpoint.rs
pub const ENDPOINT: &str = "http://collector.example.test:4318";
```

A snippet is a fragment and is not compiled:

```rust
let id = format::order_id(1);
```

```rust
// BAD: println! is not structured logging
fn main() { println!("order"); }
```

Wire it together:

```rust
mod format;

fn main() {
    println!("{} {}", format::order_id(1), format::amount(9.99));
}
```
