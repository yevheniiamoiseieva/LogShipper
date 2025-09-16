package com.example.lab1;

import org.springframework.stereotype.Component;
import java.util.Scanner;

@Component
public class QuadraticCoefficients {
    private double a;
    private double b;
    private double c;

    public void inputCoefficients() {
        Scanner scanner = new Scanner(System.in);

        System.out.println("Enter coefficients for quadratic equation ax² + bx + c = 0");

        System.out.print("Enter coefficient a: ");
        this.a = readDouble(scanner);

        System.out.print("Enter coefficient b: ");
        this.b = readDouble(scanner);

        System.out.print("Enter coefficient c: ");
        this.c = readDouble(scanner);
    }

    private double readDouble(Scanner scanner) {
        while (true) {
            if (scanner.hasNextDouble()) {
                return scanner.nextDouble();
            } else {
                System.out.print("Error: please enter a valid number! Try again: ");
                scanner.next();
            }
        }
    }

    public double getA() { return a; }
    public double getB() { return b; }
    public double getC() { return c; }
}
