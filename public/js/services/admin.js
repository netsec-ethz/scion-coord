angular.module('scionApp')
    .factory('adminService', ["$http", "$q", function($http, $q) {
    var adminService = {
        // Get supervisor state
        me: function() {
            // $http returns a promise, which has a then function, which also returns a promise
            return $http.get('/api/me').then(function (response) {
                // The then function here is an opportunity to modify the response
                console.log(response);
                // The return value gets picked up by the then in the controller.
                return response.data;
            });
        },
    };
    return adminService;
}]);
