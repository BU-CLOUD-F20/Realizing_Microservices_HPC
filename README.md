# Realizing_Microservices_HPC
Realizing Microservices and High Performance Computing

Project Logistics:
Mentors: Dan Lambright email: dlambrig@gmail.com
Will the project be open source: yes
 
Preferred Past Experience:
Kubernetes: Valuable
Go: Valuable 
Open Source Familiarity: Valuable 

Project Overview:
Background: 

This project is a continuation of work started in last year’s cloud computing course, which was presented at Linux Vault 2019. 
 
Lustre is a distributed file system used in high performance computing (HPC). Like other open source file systems (e.g. Ceph), it can run in Kubernetes, which extends it to the microservices world. This simplifies its management, as shown by students in the 2019 cloud computing course. Those students built the foundational infrastructure to package and start Lustre using “KubeVirt” (necessary to run virtual machines in containers- for Lustre’s kernel drivers). However, they did not automate the maintenance of Luster within Kubernetes. 

In a microservices environment, certain operations should be seamless:
When an instance of Lustre crashes, a new one should be created
It should be easy to auto-scale the number of instances (up or down)
Upgrading to new Luster code should be simple
It should be manageable via a dashboard

In short, Lustre should be a well-behaved microservice.
 
The students in the 2019 course showed Lustre had little performance degradation running in Kubevirt in containers. But they only experimented with a few nodes, and also did not have time to experiment with tools common in high performance computing, such as RDMA. This semester (time/resources permitting) the team will experiment with RDMA in Kubernetes. We will use the “freeflow” scheme developed at Microsoft. It removes the overhead of the “overlay network” used by containers. 
 
Project specifics: In this project you will write Go code to create “operators.” These are custom functions that are invoked when certain procedures are called in Kubernetes. Your code will be called, e.g. when a request is made to increase or decrease the number of nodes. 

You will run Microsoft’s freeflow overlay network and experiment with RDMA. The Microsoft work is open source. This will reduce the overhead over the “overlay network”. This will eliminate one more barrier to running high performance computing workloads in a microservices environment. 

More information:
2019 github site
2019 presentation at Vault conference
Microsoft “Freeflow”
Microsoft HPC container networking


Some Technologies you will learn/use:
Go programming
microservices
high performance computing problems
Virtual networking / network overlay

## Project Description

## 1. Vision and Goals Of The Project:
This project has many different and equally intriguing aspects. It can be thought of as the continuation of [last year’s students group](https://github.com/BU-NU-CLOUD-F19/Cloud-Native_high-performance_computing/). The first aspect is the automation of running [Lustre](https://wiki.lustre.org/Main_Page) in [Kubernetes](https://kubernetes.io/docs/concepts/overview/what-is-kubernetes/). Lustre is an open-source, distributed parallel file system designed for scalability, high-performance, and high-availability. In order to achieve this we will create Golang reconciler operators that will monitor the cluster and automatically scale Lustre based on different events as additions/removals of available instances or instances crashes. As we gain portability by using Kubernetes there will be a performance deterioration due to the overlay network (more about this in [Microsoft's Freeflow](https://github.com/microsoft/Freeflow)). We plan to tackle this by either utilizing remote direct memory access (RDMA) or by utilizing the same IPC namespace between different containers hosted in the same machine in order to have shared memory access between them. This will boost the communication latency and will hopefully overcome the portability overhead.

## 2. Users/Personas Of The Project:
High Performance Computing (HPC) has previously been a tool solely for researchers and software developers. From protein folding simulations to climate predictions, HPC has since been brought to a broader range of customers due to the availability of AWS HPC, Microsoft Azure, IBM Cloud and supercomputing advancements. Cloud-native HPC’s primarily users fall within these several groups.

These groups are typically researchers whose work requires immense computational capability such as analyzing/recombining DNA sequence data, protein-folding and cross-referencing genome data. Other types of users are those who work with Big Data such as data scientists forecasting product performance. However, as HPC became readily available, we have seen HPC being used by smaller companies for weather forecasting, edge-computing etc. The staple users of HPC, who do not have a deep working knowledge of computers, are those who work within manufacturing, silicon, automotive, financial institutions, and energy. An example of a manufacturing use case would be simulations of all the relevant physics and influences to determine the real-world performance of a product.

Cloud-native HPC with Lustre benefits users whose workload requires immense storage the most (petabytes worth of data). I/O performance has a widespread impact on these types of applications because of the scalable parallel read/write capabilities of Lustre and extremely fast sharing of information between containerized workloads through RDMA. In addition, Cloud-native HPC has features such as monitoring of clusters, autodiscovery/autoscale-up operators for Lustre that enable IT/dev ops engineers to support researchers in their HPC tasks. Since Cloud-native HPC incorporates Kubernates, users are not tied to a single cloud computing vendor such as AWS. At the end of the day we believe that our main persona for this project will be those two kinds of people who interact directly with infrastructure and will fully utilize our work. 

## 3. Scope and Features Of The Project:
Continue implementing features developed by last year's student group
- Set up Kubernetes on MOC instances
- Run their command-line scripts to automate running Lustre on Kubernetes

Go Scripts to create "operators" that monitor the cluster and automate the maintenance of Luster within Kubernetes
- Create a new Lustre instance when one crashes
- Easy auto-scale of the number of instances based on new instance autodiscovery
- Simplify Lustre code upgrades

Explore RDMA principles 
- Since RDMA is not available in MOC, simulate RMDA: attempt using open source "soft rdma" software
- Experiment with sharing memory between containers on the same machine

Since we can only simulate RDMA, we may not be able to do many performance tests other than just for sharing memory between containers. We will only be managing the file system and storage aspects of this project; we will not be conducting any actual high performance computing or data analysis.

## 4. Solution Concept
Global Architectural Structure Of the Project

Below is a description of the system components that are building blocks of the architectural design:
- Container: Standard, lightweight software unit that provides isolation for code and runtime environment.
- Kubernetes: Open-source container orchestration platform, automating container operations.
- Pod: Container wrapper in Kubernetes. Kubernetes’ management of containers is based on operations on pods.
- Kubernetes node: Worker machine in Kubernetes cluster. Each node contains kubelet (a component to make sure containers are running), container runtime, and kube-proxy (a node-level network proxy).
- KubeVirt: To run and manage VMs as Kubernetes pods, and allow VMs to access pod networking and storage.
- Lustre: Open-source, parallel distributed file system, which is generally used for high performance computing.
- Operators: Custom “daemon” functions to monitor and make operations on Kubernetes nodes (e.g. create, destroy, restore, etc.).
- Freeflow: High performance container overlay network. In our project, we may use it for RDMA communication, or learn from its concept to implement our memory sharing part.

<img src="images/figure01.png?raw=true"/>

**Figure 1** presents our global architectural design of this project. Lustre nodes running inside containerized KubeVirt virtual machines. Containers are managed in Kubernetes pods, and each Kubernetes node could contain multiple pods. The operator will automatically create or destroy Kubernetes nodes according to user demands, or will restore node when one crashes. In each MOC instance, there is a memory sharing module for nodes and containers.

## 5. Acceptance criteria
The MVP is to set up Lustre and running on MOC with Kubernetes on multiple machines.

- Pick up previous work, adding and revoke Lustre components on the cloud system
- Automate Lustre scaling by writing custom Golang operators for Kubernetes
- Since MOC does not support RDMA, we can share memory between containers within the same machine. Doing software level RDMA simulation can be another choice.

## 6. Release Planning:
10/1/2020 **Demo 1**: Setup single instance on MOC
- Setup Kubernetes on single instance within a cluster on MOC
- Setup multiple machines with Kubernetes in a single cluster

10/15/2020 **Demo 2**: Multi-instance within MOC and Operator exploration
- Set up 3 different instances each with Kubernetes running on the same cluster
- Implement the first two GO operators running locally. We will firstly focus on the autodiscovery/autoscale-up operator and then on the auto-shrinking operator
- Freeflow exploration for container communication based off shared memory between containers

10/29/2020 **Demo 3**: Containers running Luster
- Adjust the first two GO operators to work on a MOC machine running Kubernetes
- Implement the third GO operator on health monitoring and respawning upon failure and deploy it on MOC
- Demonstrate previous year’s project running within MOC setup
- Exploration on feasibility of RDMA software simulation. Decision whether we will head this way or towards implementing memory sharing strategy for nodes and containers in the same MOC instance

11/12/2020 **Demo 4**: Memory Sharing
- Finalize integration of the GO operators with Kubernetes instances within MOC and demonstrate Lustre operators running
- Start implementing the decided memory sharing strategy for the Lustre nodes

12/3/2020 **Demo 5**:
- Finish implementing the decided memory sharing strategy for the Lustre nodes
