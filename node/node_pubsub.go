package node

// initPubSub initializes pub-sub
func (n *Node) initPubSub() error {
	topicName := n.chain.GetPubsubTopic()
	sub, err := n.shell.PubSubSubscribe(topicName)
	if err != nil {
		return err
	}

	n.le.WithField("pubsub-topic", topicName).Debug("subscribed to pubsub")
	n.chainSub = sub
	return nil
}
