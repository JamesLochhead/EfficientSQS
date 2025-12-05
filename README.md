```
NOTE: This is a WIP project that is not yet released.
```

# Go SQS Batch and Bin-Pack Service

A Go service to batch and, compress, and bin-pack messages in memory and then
send them to Amazon SQS. The service is intended to reduce costs associated
with API calls and enable batching of messages for efficient operation of
compute/databases.

Each message is batched, compressed, and bin-packed into a single large SQS
message that is up to **1 MiB**. If, after these processes are completed,
multiple messages are present the [SendMessageBatch
API method](https://docs.aws.amazon.com/AWSSimpleQueueService/latest/APIReference/API_SendMessageBatch.html)
is used to further reduce AWS API costs.

If your code already sends large chunks (> 64KB) of data to SQS then you
will not benefit overly much from this service and it may not be a good idea to
implement it.

The service can be run anywhere that can assume an AWS IAM role on. That's
easiest on AWS services but you could, in theory, with AWS IAM Roles Anywhere,
or AWS Greengrass, do this on any compute.

## Configuration

TODO

- Polling time.
- Draining environment variable.

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
