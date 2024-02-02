export default class Socket {
    url: string;
    socket?: WebSocket;
   
    constructor(url: string) {
      this.url = url;
    }
    
    disconnect() {
        this.socket!.close()
    }

    connect() {
        this.socket = new WebSocket(this.url);
    }

    send(message: any) {
        this.socket!.send(JSON.stringify(message))
    }
    
    on(eventName: string, callback:EventListenerOrEventListenerObject) {
        this.socket!.addEventListener(eventName, callback)
    }
}
   