package com.example.lab1;

import org.springframework.beans.factory.annotation.Autowired;
import org.springframework.stereotype.Service;

@Service
public class DiscriminantCalculator {

    private final QuadraticCoefficients coefficients;

    @Autowired
    public DiscriminantCalculator(QuadraticCoefficients coefficients) {
        this.coefficients = coefficients;
    }

    public double calculate() {
        double a = coefficients.getA();
        double b = coefficients.getB();
        double c = coefficients.getC();

        return b * b - 4 * a * c;
    }

    public void calculateAndPrint() {
        coefficients.inputCoefficients();

        double discriminant = calculate();

        System.out.println("\n=== Results ===");
        System.out.println("Equation: " + coefficients.getA() + "x² + " +
                coefficients.getB() + "x + " + coefficients.getC());
        System.out.println("Discriminant = " + discriminant);

        if (discriminant > 0) {
            System.out.println("Two real roots");
        } else if (discriminant == 0) {
            System.out.println("One real root");
        } else {
            System.out.println("No real roots");
        }
    }
}