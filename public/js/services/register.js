angular.module('scionApp')
    .factory('registerService', ["$http", "$q", function($http, $q) {
    var registerService = {
        // Get supervisor state
        register: function(registration) {
            // $http returns a promise, which has a then function, which also returns a promise
            return $http.post('/api/register', registration).then(function (response) {
                // The then function here is an opportunity to modify the response
                console.log(response);
                // The return value gets picked up by the then in the controller.
                return response.data;
            });
        },

        // list all defined processes and their status
        list: function() {
            // $http returns a promise, which has a then function, which also returns a promise
            var promise = $http.get('/supervisor/list').then(function (response) {
                // The return value gets picked up by the then in the controller.
                return response.data.procs;
            });
            // Return the promise to the controller
            return promise;
        },

        // start a specific process via its name
        start: function(process_name) {
            // $http returns a promise, which has a then function, which also returns a promise
            var promise = $http.get('/supervisor/start/' + process_name).then(function (response) {
                // The then function here is an opportunity to modify the response
                console.log(response);
                // The return value gets picked up by the then in the controller.
                return response.data;
            });
            // Return the promise to the controller
            return promise;
        },

        // stop a specific process via its name
        stop: function(process_name) {
            // $http returns a promise, which has a then function, which also returns a promise
            var promise = $http.get('/supervisor/stop/'+process_name).then(function (response) {
                // The then function here is an opportunity to modify the response
                console.log(response);
                // The return value gets picked up by the then in the controller.
                return response.data;
            });
            // Return the promise to the controller
            return promise;
        },

        // startAll processes
        startAll: function() {
            // $http returns a promise, which has a then function, which also returns a promise
            var promise = $http.get('/supervisor/start_all').then(function (response) {
                // The then function here is an opportunity to modify the response
                console.log(response);
                // The return value gets picked up by the then in the controller.
                return response.data.procs;
            });
            // Return the promise to the controller
            return promise;
        },

        // stopAll processes
        stopAll: function() {
            // $http returns a promise, which has a then function, which also returns a promise
            var promise = $http.get('/supervisor/start_all').then(function (response) {
                // The then function here is an opportunity to modify the response
                console.log(response);
                // The return value gets picked up by the then in the controller.
                return response.data.procs;
            });
            // Return the promise to the controller
            return promise;
        }
    };
    return registerService;
}]);
