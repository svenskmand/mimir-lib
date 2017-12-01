// Code generated by MockGen. DO NOT EDIT.
// Source: code.uber.internal/infra/peloton/hostmgr/offer/offerpool (interfaces: Pool)

package mocks

import (
	context "context"
	reflect "reflect"
	time "time"

	v1 "code.uber.internal/infra/peloton/.gen/mesos/v1"
	hostsvc "code.uber.internal/infra/peloton/.gen/peloton/private/hostmgr/hostsvc"
	offerpool "code.uber.internal/infra/peloton/hostmgr/offer/offerpool"
	gomock "github.com/golang/mock/gomock"
)

// MockPool is a mock of Pool interface
type MockPool struct {
	ctrl     *gomock.Controller
	recorder *MockPoolMockRecorder
}

// MockPoolMockRecorder is the mock recorder for MockPool
type MockPoolMockRecorder struct {
	mock *MockPool
}

// NewMockPool creates a new mock instance
func NewMockPool(ctrl *gomock.Controller) *MockPool {
	mock := &MockPool{ctrl: ctrl}
	mock.recorder = &MockPoolMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use
func (_m *MockPool) EXPECT() *MockPoolMockRecorder {
	return _m.recorder
}

// AddOffers mocks base method
func (_m *MockPool) AddOffers(_param0 context.Context, _param1 []*v1.Offer) {
	_m.ctrl.Call(_m, "AddOffers", _param0, _param1)
}

// AddOffers indicates an expected call of AddOffers
func (_mr *MockPoolMockRecorder) AddOffers(arg0, arg1 interface{}) *gomock.Call {
	return _mr.mock.ctrl.RecordCallWithMethodType(_mr.mock, "AddOffers", reflect.TypeOf((*MockPool)(nil).AddOffers), arg0, arg1)
}

// ClaimForLaunch mocks base method
func (_m *MockPool) ClaimForLaunch(_param0 string, _param1 bool) (map[string]*v1.Offer, error) {
	ret := _m.ctrl.Call(_m, "ClaimForLaunch", _param0, _param1)
	ret0, _ := ret[0].(map[string]*v1.Offer)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// ClaimForLaunch indicates an expected call of ClaimForLaunch
func (_mr *MockPoolMockRecorder) ClaimForLaunch(arg0, arg1 interface{}) *gomock.Call {
	return _mr.mock.ctrl.RecordCallWithMethodType(_mr.mock, "ClaimForLaunch", reflect.TypeOf((*MockPool)(nil).ClaimForLaunch), arg0, arg1)
}

// ClaimForPlace mocks base method
func (_m *MockPool) ClaimForPlace(_param0 *hostsvc.HostFilter) (map[string][]*v1.Offer, map[string]uint32, error) {
	ret := _m.ctrl.Call(_m, "ClaimForPlace", _param0)
	ret0, _ := ret[0].(map[string][]*v1.Offer)
	ret1, _ := ret[1].(map[string]uint32)
	ret2, _ := ret[2].(error)
	return ret0, ret1, ret2
}

// ClaimForPlace indicates an expected call of ClaimForPlace
func (_mr *MockPoolMockRecorder) ClaimForPlace(arg0 interface{}) *gomock.Call {
	return _mr.mock.ctrl.RecordCallWithMethodType(_mr.mock, "ClaimForPlace", reflect.TypeOf((*MockPool)(nil).ClaimForPlace), arg0)
}

// CleanReservationResources mocks base method
func (_m *MockPool) CleanReservationResources() {
	_m.ctrl.Call(_m, "CleanReservationResources")
}

// CleanReservationResources indicates an expected call of CleanReservationResources
func (_mr *MockPoolMockRecorder) CleanReservationResources() *gomock.Call {
	return _mr.mock.ctrl.RecordCallWithMethodType(_mr.mock, "CleanReservationResources", reflect.TypeOf((*MockPool)(nil).CleanReservationResources))
}

// Clear mocks base method
func (_m *MockPool) Clear() {
	_m.ctrl.Call(_m, "Clear")
}

// Clear indicates an expected call of Clear
func (_mr *MockPoolMockRecorder) Clear() *gomock.Call {
	return _mr.mock.ctrl.RecordCallWithMethodType(_mr.mock, "Clear", reflect.TypeOf((*MockPool)(nil).Clear))
}

// DeclineOffers mocks base method
func (_m *MockPool) DeclineOffers(_param0 context.Context, _param1 []*v1.OfferID) error {
	ret := _m.ctrl.Call(_m, "DeclineOffers", _param0, _param1)
	ret0, _ := ret[0].(error)
	return ret0
}

// DeclineOffers indicates an expected call of DeclineOffers
func (_mr *MockPoolMockRecorder) DeclineOffers(arg0, arg1 interface{}) *gomock.Call {
	return _mr.mock.ctrl.RecordCallWithMethodType(_mr.mock, "DeclineOffers", reflect.TypeOf((*MockPool)(nil).DeclineOffers), arg0, arg1)
}

// RemoveExpiredOffers mocks base method
func (_m *MockPool) RemoveExpiredOffers() (map[string]*offerpool.TimedOffer, int) {
	ret := _m.ctrl.Call(_m, "RemoveExpiredOffers")
	ret0, _ := ret[0].(map[string]*offerpool.TimedOffer)
	ret1, _ := ret[1].(int)
	return ret0, ret1
}

// RemoveExpiredOffers indicates an expected call of RemoveExpiredOffers
func (_mr *MockPoolMockRecorder) RemoveExpiredOffers() *gomock.Call {
	return _mr.mock.ctrl.RecordCallWithMethodType(_mr.mock, "RemoveExpiredOffers", reflect.TypeOf((*MockPool)(nil).RemoveExpiredOffers))
}

// RescindOffer mocks base method
func (_m *MockPool) RescindOffer(_param0 *v1.OfferID) bool {
	ret := _m.ctrl.Call(_m, "RescindOffer", _param0)
	ret0, _ := ret[0].(bool)
	return ret0
}

// RescindOffer indicates an expected call of RescindOffer
func (_mr *MockPoolMockRecorder) RescindOffer(arg0 interface{}) *gomock.Call {
	return _mr.mock.ctrl.RecordCallWithMethodType(_mr.mock, "RescindOffer", reflect.TypeOf((*MockPool)(nil).RescindOffer), arg0)
}

// ResetExpiredHostSummaries mocks base method
func (_m *MockPool) ResetExpiredHostSummaries(_param0 time.Time) []string {
	ret := _m.ctrl.Call(_m, "ResetExpiredHostSummaries", _param0)
	ret0, _ := ret[0].([]string)
	return ret0
}

// ResetExpiredHostSummaries indicates an expected call of ResetExpiredHostSummaries
func (_mr *MockPoolMockRecorder) ResetExpiredHostSummaries(arg0 interface{}) *gomock.Call {
	return _mr.mock.ctrl.RecordCallWithMethodType(_mr.mock, "ResetExpiredHostSummaries", reflect.TypeOf((*MockPool)(nil).ResetExpiredHostSummaries), arg0)
}

// ReturnUnusedOffers mocks base method
func (_m *MockPool) ReturnUnusedOffers(_param0 string) error {
	ret := _m.ctrl.Call(_m, "ReturnUnusedOffers", _param0)
	ret0, _ := ret[0].(error)
	return ret0
}

// ReturnUnusedOffers indicates an expected call of ReturnUnusedOffers
func (_mr *MockPoolMockRecorder) ReturnUnusedOffers(arg0 interface{}) *gomock.Call {
	return _mr.mock.ctrl.RecordCallWithMethodType(_mr.mock, "ReturnUnusedOffers", reflect.TypeOf((*MockPool)(nil).ReturnUnusedOffers), arg0)
}