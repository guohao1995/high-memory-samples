# PHP Memory Stress Test Application
# Compatible with Azure App Service (Linux) and Azure Container Apps

FROM php:8.3-apache

# Configure PHP for high memory usage
RUN echo "memory_limit = 2G" > /usr/local/etc/php/conf.d/memory.ini && \
    echo "max_execution_time = 0" >> /usr/local/etc/php/conf.d/memory.ini

# Configure Apache
RUN a2enmod rewrite

# Set Apache to listen on port 8080
RUN sed -i 's/Listen 80/Listen 8080/' /etc/apache2/ports.conf && \
    sed -i 's/:80/:8080/' /etc/apache2/sites-available/000-default.conf

# Configure Apache for container health checks
ENV APACHE_RUN_USER=www-data \
    APACHE_RUN_GROUP=www-data \
    APACHE_LOG_DIR=/var/log/apache2 \
    APACHE_PID_FILE=/var/run/apache2/apache2.pid \
    APACHE_RUN_DIR=/var/run/apache2 \
    APACHE_LOCK_DIR=/var/lock/apache2

# Create necessary directories
RUN mkdir -p /var/run/apache2 /var/lock/apache2

# Copy application code
COPY index.php .htaccess /var/www/html/

# Configure directory for clean URLs
RUN echo '<Directory /var/www/html>\n\
    AllowOverride All\n\
    Require all granted\n\
</Directory>' > /etc/apache2/conf-available/custom.conf && \
    a2enconf custom

# Set proper permissions
RUN chown -R www-data:www-data /var/www/html

# Expose port
EXPOSE 8080

# Health check for container orchestration
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
    CMD curl --fail http://localhost:8080/ || exit 1

# Start Apache
CMD ["apache2-foreground"]
