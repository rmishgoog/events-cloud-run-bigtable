### Ingest timeseries event data into Cloud BigTable with Google Cloud Pub/Sub and Google Cloud Run

*Google Cloud BigTable is Google's sparsely populated database which can scale to billions of rows and thousands of columns, enabling you to store terabytes or even petabytes of data at scale. BigTable is a "NoSQL", key-value store which is ideal for storing large amount of "single-key" data, supporting a high throughput at low read/wrire latencies. A BigTable has only one index, that is the row key which will be used to look up the data, there are plenty of guidelines on how you should choose your row keys and design your schema, but broadly speaking, consider your query patterns for the store data and that should be the first thing that should guide your row keys and schema. Please refer to Google's documentation for more insights into BigTable design considerations.*

https://cloud.google.com/bigtable/docs/overview

*BigTable's unique backend architecture gives it many competitive advantages such as:*

- *Incredibly high scalability.*
- *No server management for you as a user, BigTable is fully managed by Google.*
- *Autoscaling.*
- *Global footprints, you can have big table clusters in one or more unique zones in a region or even in another zone in another region, all under a single BigTable instance,replication starts automatically without any extra configurations, making your data available across the geographies.*
- *Resize your clusters without any downtime.*
- *Simple administration, all updates, upgrades and restarts are hadnled seamlessly, ensuring data durability during such events.*

*BigTable makes a great choice for storing a variety of data:*
- *Time-series data*
- *Marketing data*
- *Financial data such as stock prices and currency exchange rates*
- *IoT*
- *Graph data, showing relationship between entities*

*In this tutorial we will use BigTable to implement a time-series database, where events from an external source arrive at regular intervals of time, are ingested into Cloud Pub/Sub and pushed to a Cloud Run API which then handles the events and persist the data into BigTable.*

*Our tutorial simulates a climate/weather monitoring office, they get events through some sensors placed in several big cities and towns across the United States, with each event describing the climatic condition at a given point in time, such as temperature, air pressure, pollution score etc. These sensors emit these events several times (per-minute) during the course of a week and gets logged into BigTable. At the end of the week, the designated 'climate watchdog' can use this data for several analytical and other use cases.*

*In this time bucket pattern, you add new cells to existing columns when you write a new event. This pattern lets you take advantage of Bigtable's ability to let you store multiple timestamped cells in a given row and column. It's important to specify garbage collection rules when you use this pattern. The commands that I have included will show how do you do that when you create a table via the CLI.*

*Also, pay attention to the row key that will be used, it wil be of the format state#county#city#week, where # is the de-limiter, this row key will act as an identifier of the week's data. At the end of the week, each column in each row has one measurement for each minute of the week, or 10,080 cells (if your garbage collection policy allows it).*

*Let's begin provisioning some stuff and see our application/API in action!*

*First, make sure that you have an active GCP project with a valid billing account, also ensure that you have right roles and permissions in order to provision the resources we will need for this tutorial. For the sake of simplicity, use an account with 'Project Owner' or 'Project Editor' role assigned to it, however, always keep in mind that convinience over security is never a good practice and I highly recommend that in your production environments, always use the principle of least privileges, assigning the identities exactly what they need, no more and no less.*

*On your workstation, if you do not have the gcloud CLI installed, please go ahead and install it, if you do, you can move on to the next step.*

```
gcloud auth login
```







