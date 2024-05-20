package hygon

import (
	"errors"
	"flag"
	"strings"

	"github.com/Project-HAMi/HAMi/pkg/api"
	"github.com/Project-HAMi/HAMi/pkg/util"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/klog/v2"
)

type DCUDevices struct {
}

const (
	HandshakeAnnos     = "hami.io/node-handshake-dcu"
	RegisterAnnos      = "hami.io/node-dcu-register"
	HygonDCUDevice     = "DCU"
	HygonDCUCommonWord = "DCU"
	DCUInUse           = "hygon.com/use-dcutype"
	DCUNoUse           = "hygon.com/nouse-dcutype"
)

var (
	HygonResourceCount  string
	HygonResourceMemory string
	HygonResourceCores  string
)

func InitDCUDevice() *DCUDevices {
	util.InRequestDevices[HygonDCUDevice] = "hami.io/vgpu-devices-to-allocate"
	util.SupportDevices[HygonDCUDevice] = "hami.io/vgpu-devices-allocated"
	util.HandshakeAnnos[HygonDCUDevice] = HandshakeAnnos

	return &DCUDevices{}
}

func (dev *DCUDevices) ParseConfig(fs *flag.FlagSet) {
	fs.StringVar(&HygonResourceCount, "dcu-name", "hygon.com/dcunum", "dcu resource count")
	fs.StringVar(&HygonResourceMemory, "dcu-memory", "hygon.com/dcumem", "dcu memory resource")
	fs.StringVar(&HygonResourceCores, "dcu-cores", "hygon.com/dcucores", "dcu core resource")
}

func (dev *DCUDevices) MutateAdmission(ctr *corev1.Container) bool {
	_, ok := ctr.Resources.Limits[corev1.ResourceName(HygonResourceCount)]
	return ok
}

func checkDCUtype(annos map[string]string, cardtype string) bool {
	inuse, ok := annos[DCUInUse]
	if ok {
		if !strings.Contains(inuse, ",") {
			if strings.Contains(strings.ToUpper(cardtype), strings.ToUpper(inuse)) {
				return true
			}
		} else {
			for _, val := range strings.Split(inuse, ",") {
				if strings.Contains(strings.ToUpper(cardtype), strings.ToUpper(val)) {
					return true
				}
			}
		}
		return false
	}
	nouse, ok := annos[DCUNoUse]
	if ok {
		if !strings.Contains(nouse, ",") {
			if strings.Contains(strings.ToUpper(cardtype), strings.ToUpper(nouse)) {
				return false
			}
		} else {
			for _, val := range strings.Split(nouse, ",") {
				if strings.Contains(strings.ToUpper(cardtype), strings.ToUpper(val)) {
					return false
				}
			}
		}
		return true
	}
	return true
}

func (dev *DCUDevices) GetNodeDevices(n corev1.Node) ([]*api.DeviceInfo, error) {
	devEncoded, ok := n.Annotations[RegisterAnnos]
	if !ok {
		return []*api.DeviceInfo{}, errors.New("annos not found " + RegisterAnnos)
	}
	nodedevices, err := util.DecodeNodeDevices(devEncoded)
	if err != nil {
		klog.ErrorS(err, "failed to decode node devices", "node", n.Name, "device annotation", devEncoded)
		return []*api.DeviceInfo{}, err
	}
	if len(nodedevices) == 0 {
		klog.InfoS("no gpu device found", "node", n.Name, "device annotation", devEncoded)
		return []*api.DeviceInfo{}, errors.New("no gpu found on node")
	}
	devDecoded := util.EncodeNodeDevices(nodedevices)
	klog.V(5).InfoS("nodes device information", "node", n.Name, "nodedevices", devDecoded)
	return nodedevices, nil
}

func (dev *DCUDevices) NodeCleanUp(nn string) error {
	return util.MarkAnnotationsToDelete(HygonDCUDevice, nn)
}

func (dev *DCUDevices) CheckHealth(devType string, n *corev1.Node) (bool, bool) {
	return util.CheckHealth(devType, n)
}

func (dev *DCUDevices) CheckType(annos map[string]string, d util.DeviceUsage, n util.ContainerDeviceRequest) (bool, bool, bool) {
	if strings.Compare(n.Type, HygonDCUDevice) == 0 {
		return true, checkDCUtype(annos, d.Type), false
	}
	return false, false, false
}

func (dev *DCUDevices) GenerateResourceRequests(ctr *corev1.Container) util.ContainerDeviceRequest {
	klog.Info("Counting dcu devices")
	dcuResourceCount := corev1.ResourceName(HygonResourceCount)
	dcuResourceMem := corev1.ResourceName(HygonResourceMemory)
	dcuResourceCores := corev1.ResourceName(HygonResourceCores)
	v, ok := ctr.Resources.Limits[dcuResourceCount]
	if !ok {
		v, ok = ctr.Resources.Requests[dcuResourceCount]
	}
	if ok {
		if n, ok := v.AsInt64(); ok {
			klog.Info("Found dcu devices")
			memnum := 0
			mem, ok := ctr.Resources.Limits[dcuResourceMem]
			if !ok {
				mem, ok = ctr.Resources.Requests[dcuResourceMem]
			}
			if ok {
				memnums, ok := mem.AsInt64()
				if ok {
					memnum = int(memnums)
				}
			}
			corenum := int32(0)
			core, ok := ctr.Resources.Limits[dcuResourceCores]
			if !ok {
				core, ok = ctr.Resources.Requests[dcuResourceCores]
			}
			if ok {
				corenums, ok := core.AsInt64()
				if ok {
					corenum = int32(corenums)
				}
			}

			mempnum := 0
			if memnum == 0 {
				mempnum = 100
			}

			return util.ContainerDeviceRequest{
				Nums:             int32(n),
				Type:             HygonDCUDevice,
				Memreq:           int32(memnum),
				MemPercentagereq: int32(mempnum),
				Coresreq:         corenum,
			}
		}
	}
	return util.ContainerDeviceRequest{}
}

func (dev *DCUDevices) PatchAnnotations(annoinput *map[string]string, pd util.PodDevices) map[string]string {
	devlist, ok := pd[HygonDCUDevice]
	if ok && len(devlist) > 0 {
		deviceStr := util.EncodePodSingleDevice(devlist)
		(*annoinput)[util.InRequestDevices[HygonDCUDevice]] = deviceStr
		(*annoinput)[util.SupportDevices[HygonDCUDevice]] = deviceStr
		klog.V(5).Infof("pod add notation key [%s], values is [%s]", util.InRequestDevices[HygonDCUDevice], deviceStr)
		klog.V(5).Infof("pod add notation key [%s], values is [%s]", util.SupportDevices[HygonDCUDevice], deviceStr)
	}

	return *annoinput
}
