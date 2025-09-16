package com.example.lab1;

import org.springframework.beans.factory.annotation.Autowired;
import org.springframework.boot.CommandLineRunner;
import org.springframework.boot.SpringApplication;
import org.springframework.boot.autoconfigure.SpringBootApplication;

@SpringBootApplication
public class Lab1Application implements CommandLineRunner {

    private final DiscriminantCalculator calculator;

    @Autowired
    public Lab1Application(DiscriminantCalculator calculator) {
        this.calculator = calculator;
    }

    public static void main(String[] args) {
        SpringApplication.run(Lab1Application.class, args);
    }

    @Override
    public void run(String... args) throws Exception {
        System.out.println("=== Discriminant Calculator ===");
        calculator.calculateAndPrint();
    }
}