package core

import (
	"context"
	"strings"
	"testing"
	"time"
)

// 局域网实测：SSDP 发现（无设备时不失败，只验证不崩、可搜索）。
func TestCastDiscoverLocal(t *testing.T) {
	if testing.Short() {
		t.Skip("short 模式跳过局域网探测")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 6*time.Second)
	defer cancel()
	devices, err := discoverCastDevices(ctx)
	if err != nil {
		t.Fatalf("discover: %v", err)
	}
	for _, d := range devices {
		t.Logf("发现: %s | %s | ctrl=%s", d.Name, d.Server, d.ControlURL)
	}
	if len(devices) == 0 {
		t.Log("本网络未发现 DLNA 设备（正常，仅功能验证）")
	}
}

func TestBuildCastSOAPBody(t *testing.T) {
	body := buildCastSOAPBody("http://a/b.m3u8", "A&B <>剧")
	for _, want := range []string{`SetAVTransportURI`, `http://a/b.m3u8`, `A&amp;B &lt;&gt;剧`, `InstanceID>0<`} {
		if !strings.Contains(body, want) {
			t.Errorf("SOAP body 缺少 %q:\n%s", want, body)
		}
	}
}

func TestCastDeviceDescriptionParse(t *testing.T) {
	XML := `<root xmlns="urn:schemas-upnp-org:device-1-0">
	<device>
		<friendlyName>客厅电视</friendlyName>
		<serviceList>
			<service>
				<serviceType>urn:schemas-upnp-org:service:AVTransport:1</serviceType>
				<controlURL>/ctl/avt</controlURL>
			</service>
		</serviceList>
	</device>
	</root>`
	server := NewCastDescriptionServerForTest(t, XML)
	defer server.Close()
	device := castDevice{ID: "u1", Location: server.URL + "/dd.xml", Server: "test"}
	parsed, err := loadCastDeviceDescription(context.Background(), device)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if parsed.Name != "客厅电视" {
		t.Errorf("name = %s", parsed.Name)
	}
	if parsed.ControlURL != server.URL+"/ctl/avt" {
		t.Errorf("controlURL = %s", parsed.ControlURL)
	}
}
