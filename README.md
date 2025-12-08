# Efficient SQS

A Go service to batch and bin-pack messages in-memory and then send them to
Amazon SQS. The service is intended to reduce costs associated with API calls
and enable batching of messages for efficient operation of compute/databases.

Each message is batched and bin-packed into a single large SQS message that is
up to **1 MiB**. If, after these processes are completed, multiple messages are
present the
[SendMessageBatch API method](https://docs.aws.amazon.com/AWSSimpleQueueService/latest/APIReference/API_SendMessageBatch.html)
is used to further reduce AWS API costs.

If your code already sends large chunks (> 64KB) of data to SQS then you will
not benefit overly much from this service from a purely SQS cost reduction
perspective.

The service can be run anywhere that can assume an AWS IAM role on. That's
easiest on AWS services but you could, in theory, with AWS IAM Roles Anywhere,
or AWS Greengrass, do this on any compute.

Does not care about order, so this is not suitable for FIFO queues.

Additionally, inherently standard SQS queues output a small number of duplicate
messages; your code must be able to deal with this.

## Configuration

The application can be configured by a single `efficient_sqs_config.toml` that
is used by both processes. All keys are top-level.

The configuration file can be located in the same directory as the binaries,
the directories above, or at `/etc/efficient/sqs/`.

```
port
```

The port that the Gin web server should listen on.

```
pollingMs
```

How frequently the second process should poll the Redis queue in milliseconds.

```
mode
```

`"release"` or `"debug"`. Set the application, including Gin, to release or
debug mode.

```
sqsQueueName
```

The name of the SQS queue to push messages to.

```
redisHost
```

The host for Redis. Not required unless you run Redis on a different host.
Defaults to `localhost`.

```
redisPort
```

The port for Redis. Not required unless you run Redis on a different port.
Defaults to `6379`.

```
redisQueueName
```

The Redis queue name to use. Not required unless you want to use a different
queue name. Defaults to `queue_b1946ac92`.

### Critical

It is critical to tune `pollingMs` for your application.

If `pollingMs` too low, then you won't collect enough messages to make the
batching/bin-packing worthwhile.

Conversely, if `pollingMs` is too high, user experience might be impacted, you
will lose more data during a disaster, and your compute will need more memory.

`separatingCharacters` is used to separate your messages as they are bin-packed
into a single message. Messages that initially contain the separating characters
are rejected; therefore it is critical to set them to characters that your
messages should ideally **never** container.

When consuming messages from the SQS queue, you will need to search for
separating characters to unpack each message.

### IAM role

The application assumes you have an appropriate IAM role with permissions to
send messages to SQS and AWS region set.

Here's a minimal IAM policy document:

```json
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Sid": "SqsSendMessage",
      "Effect": "Allow",
      "Action": "sqs:SendMessage",
      "Resource": "arn:aws:sqs:AWS_REGION:AWS_ACCOUNT:QUEUE_NAME"
    }
  ]
}
```

## Benefits

Outside of the free tier, every SQS request has a cost. An SQS message can be
at most **1 MiB** at the time of writing. Each 64 KB chunk is billed as a request.
Batching and bin-packing requests into chunks saves on costs by reducing the
number of SQS messages sent tothe AWS API.

See: [https://aws.amazon.com/sqs/pricing/](https://aws.amazon.com/sqs/pricing/)

On its own, SQS pricing is perhaps not enough to justify the cost of
implementing and maintaining a service like this unless you're dealing with
billions of requests per month.

Let's assume you're making 1.001 billion requests in `us-east-2` - currently
priced at `$0.40` / 1 million requests and this service (perhaps) reduces API
calls by 80%.

You might save:

```
((1001000000 - 1000000) / 1000000) * $0.40 * 0.8 = $320 per month
```

More importantly, when writing a highly scalable web application, **disk IOPS**
usually becomes a limiting factor. Disks are usually glacially slow compared to
other parts of a computer system, such as the CPU cache. Writing small commits
to your database is an inefficient use of a finite resource and can cause cost,
performance, and scalability issues. Batching writes in an SQS queue before
committing them to a database enables database engines to operate much more
efficiently and make the best use of available disk IOPS.

Traditional SQL databases usually have a single writer node, even when running
in a cluster. We (humanity) have not yet perfectly solved how to scale
databases vertically online with no downtime, caveats, or degradation of
service. Queueing up writes to a database enables **drip-feeding** of writes
should the load on a service exceed what a single writer node can handle. The
service will hopefully manage, for a short time, with little noticeable
degradation. This buffer period might also enable vertical scaling to take
place, should the demand persist.

Additionally, if the database needs to be made read-only — for example, during
a schema migration — a service that uses queues may be able to continue
accepting writes temporarily.

NoSQL databases can overcome limitations with single writer nodes but come with
other complications such as more complex or expensive joins, insufficient
consistency guarantees, lack of appropriate data types, and limited record
sizes.

Ultimately, this project is far more about ensuring the data layer is less of a
bottleneck to scaling and users are not impacted by service degradation
associated with overly utilized databases, than it is about saving SQS costs.

## Notes

- APIs that use this service are writing **asynchronously** by definition; this
  is not appropriate for every use case. For example, for financial data you
  may want to guarantee that the write actually happened (with a successful API
  call meaning the write is complete).
- By design, this service **slows down API requests slightly** to enable them
  to be batched.
- Requests are temporarily held in memory on a single, **non–highly available**
  compute instance. That’s fine for many (I dare say even most) use cases, but
  not all.
- You should ensure requests are authenticated, authorized, and validated prior
  to sending them to this service. Alternatively, you can fork this code and
  write that code yourself.
- Use firewalls to restrict access to this service.

## Architecture

- There are three processes that independently.
- A web server process (using Gin) put requests into a Redis queue.
- Redis running as a standalone process (non-HA).
- A polling process that pops items from the Redis queue, bin-packs, batches
  them, and then sends them to SQS.

## Production

In production, I would suggest running this as a single pod or ECS task.

Each pod would contain a container each for Redis, `intermediate_q_process`,
and `store_sqs_process`.

You might add a fourth container into the mix, built internally, to validate
and auth requests before they are sent to `intermediate_q_process`, or
you might run everything behind an API Gateway.

Alternatively, you could fork this repository and add validation/auth.
