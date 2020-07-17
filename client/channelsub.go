package client

type ChannelSubscription struct{}

func (sub *ChannelSubscription) Unsubscribe() {

}

func (sub *ChannelSubscription) Err() <-chan error {
	return make(chan error)
}
