public interface Shape {
    double area();
}

public class Calculator implements Shape {
    int add(int a, int b) { return a + b; }

    Calculator(int seed) { this.seed = seed; }

    public double area() { return 0.0; }

    int hidden(int x) { return x; }
}
