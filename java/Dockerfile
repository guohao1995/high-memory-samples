# Java Memory Stress Test Application
# Compatible with Azure App Service (Linux) and Azure Container Apps

# Build stage
FROM maven:3.9-eclipse-temurin-21 AS build
WORKDIR /app

# Copy pom.xml and download dependencies
COPY pom.xml .
RUN mvn dependency:go-offline -B

# Copy source code and build
COPY src ./src
RUN mvn package -DskipTests -B

# Runtime stage
FROM eclipse-temurin:21-jre-jammy AS runtime

# Set environment variables – let the JVM detect the container memory limit
ENV JAVA_OPTS="-XX:+UseContainerSupport -XX:MaxRAMPercentage=80.0 -XX:+UseG1GC"

WORKDIR /app

# Copy the built JAR
COPY --from=build /app/target/memory-stress.jar app.jar

# Create non-root user for security
RUN addgroup --system --gid 1001 javauser && \
    adduser --system --uid 1001 --gid 1001 javauser && \
    chown -R javauser:javauser /app

USER javauser

# Expose port
EXPOSE 8080

# Health check for container orchestration
HEALTHCHECK --interval=30s --timeout=3s --start-period=30s --retries=3 \
    CMD curl --fail http://localhost:8080/ || exit 1

# Run the application with configurable JVM options
ENTRYPOINT ["sh", "-c", "java $JAVA_OPTS -jar app.jar"]
