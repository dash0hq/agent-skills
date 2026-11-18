# Broken Rust crate fixture

```toml
[dependencies]
itoa = "=1.0.15"
```

```rust
// src/format.rs
pub fn order_id(n: u32) -> String {
    let mut buffer = itoa::Buffer::new();
    format!("TEST-{:0>4}", buffer.format(n))
}
```

```rust
mod format;

fn main() {
    let router = app();
    println!("{} {:?}", format::order_id(1), router);
}
```
