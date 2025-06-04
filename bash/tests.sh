docker-compose build

# kafka tests
docker-compose down -v
docker-compose up -d
sleep 10
docker-compose down statistics
docker-compose up tests_kafka_producer

# e2e tests
docker-compose down -v
docker-compose up -d
sleep 10
docker-compose up tests_e2e