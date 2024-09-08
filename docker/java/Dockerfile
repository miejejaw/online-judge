# Base image for Java
FROM openjdk:17-slim

# Set the working directory
WORKDIR /app

# The actual code will be mounted at runtime
CMD ["sh", "-c", "javac Main.java && java Main"]
