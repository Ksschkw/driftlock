struct Loader {
    func fetch(url: String) -> Data {
        return Data()
    }

    func onDone(handler: (Int) -> Void, label: String) {
        handler(0)
    }
}
