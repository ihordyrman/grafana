package apiserver

import (
	"fmt"
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/labels"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/apiserver/pkg/server/dynamiccertificates"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/klog/v2"

	v0alpha1 "github.com/grafana/grafana/pkg/aggregator/apis/aggregation/v0alpha1"
	informers "github.com/grafana/grafana/pkg/aggregator/generated/informers/externalversions/aggregation/v0alpha1"
	listers "github.com/grafana/grafana/pkg/aggregator/generated/listers/aggregation/v0alpha1"
)

// APIHandlerManager defines the behaviour that an API handler should have.
type APIHandlerManager interface {
	AddDataPlaneService(dataPlaneService *v0alpha1.DataPlaneService) error
	RemoveDataPlaneService(dataPlaneServiceName string)
}

// DataPlaneServiceRegistrationController is responsible for registering and removing API services.
type DataPlaneServiceRegistrationController struct {
	apiHandlerManager APIHandlerManager

	dataPlaneServiceLister listers.DataPlaneServiceLister
	dataPlaneServiceSynced cache.InformerSynced

	// To allow injection for testing.
	syncFn func(key string) error

	queue workqueue.RateLimitingInterface
}

var _ dynamiccertificates.Listener = &DataPlaneServiceRegistrationController{}

// NewDataPlaneServiceRegistrationController returns a new DataPlaneServiceRegistrationController.
func NewDataPlaneServiceRegistrationController(dataPlaneServiceInformer informers.DataPlaneServiceInformer, apiHandlerManager APIHandlerManager) *DataPlaneServiceRegistrationController {
	c := &DataPlaneServiceRegistrationController{
		apiHandlerManager:      apiHandlerManager,
		dataPlaneServiceLister: dataPlaneServiceInformer.Lister(),
		dataPlaneServiceSynced: dataPlaneServiceInformer.Informer().HasSynced,
		queue:                  workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "DataPlaneServiceRegistrationController"),
	}

	dataPlaneServiceInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    c.addDataPlaneService,
		UpdateFunc: c.updateDataPlaneService,
		DeleteFunc: c.deleteDataPlaneService,
	})

	c.syncFn = c.sync

	return c
}

func (c *DataPlaneServiceRegistrationController) sync(key string) error {
	dataPlaneService, err := c.dataPlaneServiceLister.Get(key)
	if apierrors.IsNotFound(err) {
		c.apiHandlerManager.RemoveDataPlaneService(key)
		return nil
	}
	if err != nil {
		return err
	}

	return c.apiHandlerManager.AddDataPlaneService(dataPlaneService)
}

// Run starts DataPlaneServiceRegistrationController which will process all registration requests until stopCh is closed.
func (c *DataPlaneServiceRegistrationController) Run(stopCh <-chan struct{}, handlerSyncedCh chan<- struct{}) {
	defer utilruntime.HandleCrash()
	defer c.queue.ShutDown()

	klog.Info("Starting DataPlaneServiceRegistrationController")
	defer klog.Info("Shutting down DataPlaneServiceRegistrationController")

	if !cache.WaitForCacheSync(stopCh, c.dataPlaneServiceSynced) {
		return
	}

	/// initially sync all DataPlaneServices to make sure the proxy handler is complete
	if err := wait.PollImmediateUntil(time.Second, func() (bool, error) {
		services, err := c.dataPlaneServiceLister.List(labels.Everything())
		if err != nil {
			utilruntime.HandleError(fmt.Errorf("failed to initially list DataPlaneServices: %v", err))
			return false, nil
		}
		for _, s := range services {
			if err := c.apiHandlerManager.AddDataPlaneService(s); err != nil {
				utilruntime.HandleError(fmt.Errorf("failed to initially sync DataPlaneService %s: %v", s.Name, err))
				return false, nil
			}
		}
		return true, nil
	}, stopCh); err == wait.ErrWaitTimeout {
		utilruntime.HandleError(fmt.Errorf("timed out waiting for proxy handler to initialize"))
		return
	} else if err != nil {
		panic(fmt.Errorf("unexpected error: %v", err))
	}
	close(handlerSyncedCh)

	// only start one worker thread since its a slow moving API and the aggregation server adding bits
	// aren't threadsafe
	go wait.Until(c.runWorker, time.Second, stopCh)

	<-stopCh
}

func (c *DataPlaneServiceRegistrationController) runWorker() {
	for c.processNextWorkItem() {
	}
}

// processNextWorkItem deals with one key off the queue.  It returns false when it's time to quit.
func (c *DataPlaneServiceRegistrationController) processNextWorkItem() bool {
	key, quit := c.queue.Get()
	if quit {
		return false
	}
	defer c.queue.Done(key)

	err := c.syncFn(key.(string))
	if err == nil {
		c.queue.Forget(key)
		return true
	}

	utilruntime.HandleError(fmt.Errorf("%v failed with : %v", key, err))
	c.queue.AddRateLimited(key)

	return true
}

func (c *DataPlaneServiceRegistrationController) enqueueInternal(obj *v0alpha1.DataPlaneService) {
	key, err := cache.DeletionHandlingMetaNamespaceKeyFunc(obj)
	if err != nil {
		klog.Errorf("Couldn't get key for object %#v: %v", obj, err)
		return
	}

	c.queue.Add(key)
}

func (c *DataPlaneServiceRegistrationController) addDataPlaneService(obj interface{}) {
	castObj := obj.(*v0alpha1.DataPlaneService)
	klog.V(4).Infof("Adding %s", castObj.Name)
	c.enqueueInternal(castObj)
}

func (c *DataPlaneServiceRegistrationController) updateDataPlaneService(obj, _ interface{}) {
	castObj := obj.(*v0alpha1.DataPlaneService)
	klog.V(4).Infof("Updating %s", castObj.Name)
	c.enqueueInternal(castObj)
}

func (c *DataPlaneServiceRegistrationController) deleteDataPlaneService(obj interface{}) {
	castObj, ok := obj.(*v0alpha1.DataPlaneService)
	if !ok {
		tombstone, ok := obj.(cache.DeletedFinalStateUnknown)
		if !ok {
			klog.Errorf("Couldn't get object from tombstone %#v", obj)
			return
		}
		castObj, ok = tombstone.Obj.(*v0alpha1.DataPlaneService)
		if !ok {
			klog.Errorf("Tombstone contained object that is not expected %#v", obj)
			return
		}
	}
	klog.V(4).Infof("Deleting %q", castObj.Name)
	c.enqueueInternal(castObj)
}

// Enqueue queues all data plane services to be rehandled.
// This method is used by the controller to notify when the proxy cert content changes.
func (c *DataPlaneServiceRegistrationController) Enqueue() {
	dataPlaneServices, err := c.dataPlaneServiceLister.List(labels.Everything())
	if err != nil {
		utilruntime.HandleError(err)
		return
	}
	for _, dataPlaneService := range dataPlaneServices {
		c.addDataPlaneService(dataPlaneService)
	}
}
