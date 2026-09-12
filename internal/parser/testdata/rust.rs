pub struct Holder<'a> {
    inner: &'a str,
}

pub fn get<'a>(x: &'a str) -> &'a str {
    x
}

impl Widget {
    pub fn new() -> Self { Widget }

    pub fn build() -> Self { Widget }
}

impl Gadget {
    pub fn build() -> Self { Gadget }
}

impl<T> Store<T> for Foo<T> {
    pub fn put(&self, value: T) -> Result<(), Error> {
        Ok(())
    }
}
