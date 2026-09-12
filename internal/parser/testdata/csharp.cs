public class Repo {
    List<Item> All() { return items; }

    void Save(int id) => store.Put(id);

    int Count => items.Count;

    public string Name { get; set; }
}
