package sample;

public interface Shape {
    double area();
    int sides();
}

public class Calculator implements Shape {
    int add(int a, int b) {
        return a + b;
    }

    Calculator(int seed) {
        this.seed = seed;
    }

    public double area() { return 0.0; }

    public int sides() { return 4; }

    private void log(String msg) {
        System.out.println(msg);
    }
}
