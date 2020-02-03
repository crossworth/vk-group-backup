## VK Group Backup

Docker images for making backup of all topics from a VK Group/Community.

To run it you will need an `RabbitMQ` instance, a `MongoDB` and Docker.

### Steps
1. Login on Github docker registry
2. Start the `queue all topics` image, it will  get all the topics id from the given community and send to rabbitmq.
3. Start the `worker` image, it will save a topic at time from rabbitmq to mongodb.


Example running the `queue all topics`
```shell script
docker run -d \
-e VK_GROUP_ID=0 \ 
-e VK_EMAIL=email@goes.here \
-e VK_PASSWORD=password  \
-e RABBITMQ_SERVER=amqp://localhost:5672 \ 
-e RABBITMQ_QUEUE_NAME=queue-name \
docker.pkg.github.com/crossworth/vk-group-backup/queue-all-topics:latest
```

Example running the `worker`
```shell script
docker run -d \
-e MONGO_SERVER=mongodb://localhost:27017 \ 
-e MONGO_DATABASE=mongodb \
-e VK_GROUP_ID=0 \ 
-e VK_EMAIL=email@goes.here \
-e VK_PASSWORD=password  \
-e RABBITMQ_SERVER=amqp://localhost:5672 \ 
-e RABBITMQ_QUEUE_NAME=queue-name \
docker.pkg.github.com/crossworth/vk-group-backup/worker:latest
```

#### NOTES
You can change the code to use `go channels` instead of `RabbitMQ` and `JSON` instead of `MongoDB`, making it easy to deploy.

You can change/create a new process to handle the recent topics to be able to make live updates of topics, the function `GetRecentTopicsIDs` is already implemented.

You can have more then 1 worker, but you must define an environment variable called `VK_DEVICE` with `iphone`, `androind` or `wphone`.
If you try make a lot of request from the same device you will be rate limited/blocked for 2 hours.

If you are a group administrator you can use `LongPolling` from `VKAPI`, its better and easy than using this project. 

