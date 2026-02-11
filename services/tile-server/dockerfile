# Use the official Martin image
FROM ghcr.io/maplibre/martin:1.3.0

# Copy the local config file into the container
# We put it in /etc/martin/ to follow standard linux conventions
COPY config.yaml /etc/martin/config.yaml

# Run Martin using the config file
# Since the base image sets the ENTRYPOINT to "martin", 
# these arguments are passed directly to the binary.
CMD ["--config", "/etc/martin/config.yaml"]

EXPOSE 3000