package com.memorystress;

import org.springframework.boot.SpringApplication;
import org.springframework.boot.autoconfigure.SpringBootApplication;

/**
 * Memory Stress Test Application - Java/Spring Boot
 * Fills a List with byte arrays to stress the JVM Heap.
 */
@SpringBootApplication
public class MemoryStressApplication {

    public static void main(String[] args) {
        SpringApplication.run(MemoryStressApplication.class, args);
    }
}
