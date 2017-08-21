angular.module('scionApp')
    .factory('loginService', ["$http", "$q", function($http, $q) {
    var loginService = {
        // Log the user in
        login: function(user) {
            // $http returns a promise, which has a then function, which also returns a promise
            return $http.post('/api/login', user).then(function (response) {
                // The then function here is an opportunity to modify the response
                console.log(response);
                // The return value gets picked up by the then in the controller.
                return response.data;
            });
        },
        logout: function() {
            // $http returns a promise, which has a then function, which also returns a promise
            return $http.post('/api/logout').then(function (response) {
                // The then function here is an opportunity to modify the response
                console.log(response);
                // The return value gets picked up by the then in the controller.
                return response.data;
            });
        }
    };
    return loginService;
}]);
