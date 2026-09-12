pub fn get<'a>(x: &'a str) -> &'a str {
    x
}

pub struct Holder<'a> {
    inner: &'a str,
}

impl Widget {
    pub fn new() -> Self { Widget }
    pub fn build() -> Self { Widget }
}

impl Gadget {
    pub fn build() -> Self { Gadget }
}

fn private_fn() {}
