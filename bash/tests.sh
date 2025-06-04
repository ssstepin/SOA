docker-compose down -v
docker-compose build
docker-compose up -d
sleep 10
docker-compose down statistics
docker-compose up tests_kafka_producer